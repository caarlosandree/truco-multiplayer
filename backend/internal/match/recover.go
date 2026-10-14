package match

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
)

// Recovered é uma partida em andamento reconstruída para a sala (Spec 3) retomar.
type Recovered struct {
	Record  replay.Record
	State   engine.GameState
	NextPly int
}

// Recover reconstrói as partidas em andamento após um restart. Partidas irrecuperáveis
// são abandonadas e as que já tinham terminado são finalizadas; o servidor sobe mesmo assim.
func (s *Store) Recover(ctx context.Context, log *slog.Logger) ([]Recovered, error) {
	ids, err := s.q.ListInProgressMatchIDs(ctx)
	if err != nil {
		return nil, err
	}
	var out []Recovered
	for _, id := range ids {
		rec, err := s.Load(ctx, id)
		var state engine.GameState
		if err == nil {
			state, err = replay.Rebuild(rec)
		}
		var corrupt *replay.CorruptLogError
		switch {
		case errors.Is(err, replay.ErrIncompatibleVersion):
			if err := s.abandonUnrecoverable(ctx, log, id, "incompatible_version", err); err != nil {
				return nil, err
			}
			continue
		case errors.As(err, &corrupt):
			if err := s.abandonUnrecoverable(ctx, log, id, "corrupt_log", err); err != nil {
				return nil, err
			}
			continue
		case err != nil:
			return nil, err
		}
		if state.Phase == engine.PhaseGameOver {
			if err := s.Finish(ctx, id); err != nil {
				return nil, err
			}
			continue
		}
		out = append(out, Recovered{Record: rec, State: state, NextPly: len(rec.Actions)})
	}
	return out, nil
}

func (s *Store) abandonUnrecoverable(ctx context.Context, log *slog.Logger, id, reason string, cause error) error {
	log.ErrorContext(ctx, "partida irrecuperável abandonada", "match_id", id, "reason", reason, "err", cause)
	return s.Abandon(ctx, id, reason)
}

// RecomputeStats refaz match_player_stats e player_stats a partir dos logs de todas as partidas finalizadas.
func (s *Store) RecomputeStats(ctx context.Context) (skipped []string, err error) {
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		skipped = nil
		q := s.q.WithTx(tx)
		if err := q.DeleteAllMatchPlayerStats(ctx); err != nil {
			return err
		}
		if err := q.DeleteAllPlayerStats(ctx); err != nil {
			return err
		}
		ids, err := q.ListFinishedMatchIDs(ctx)
		if err != nil {
			return err
		}
		now := s.now()
		for _, id := range ids {
			m, err := q.GetMatch(ctx, id)
			if err != nil {
				return err
			}
			rec, err := loadRecord(ctx, q, m)
			var corrupt *replay.CorruptLogError
			if errors.As(err, &corrupt) {
				skipped = append(skipped, id)
				continue
			}
			if err != nil {
				return err
			}
			perSeat, err := stats.Derive(rec)
			if err != nil {
				skipped = append(skipped, id)
				continue
			}
			if err := writeStats(ctx, q, id, perSeat, now); err != nil {
				return err
			}
		}
		return nil
	})
	return skipped, err
}
