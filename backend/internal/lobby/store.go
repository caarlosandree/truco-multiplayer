package lobby

import (
	"context"
	"errors"
	"fmt"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

// NewMatchStore adapta *match.Store à porta da sala, traduzindo match.ErrMatchClosed, ErrPlyConflict e ErrPlyGap.
func NewMatchStore(s *match.Store) room.MatchStore { return storeAdapter{s} }

type storeAdapter struct{ s *match.Store }

func (a storeAdapter) Create(ctx context.Context, seed uint64, dealer engine.Seat, players [4]replay.PlayerSeat) (string, error) {
	return a.s.Create(ctx, seed, dealer, players)
}

func (a storeAdapter) AppendAction(ctx context.Context, id string, ply int, act engine.Action, src replay.Source) error {
	return translate(a.s.AppendAction(ctx, id, ply, act, src))
}

func (a storeAdapter) Finish(ctx context.Context, id string) error {
	return translate(a.s.Finish(ctx, id))
}

func (a storeAdapter) Abandon(ctx context.Context, id, reason string) error {
	return translate(a.s.Abandon(ctx, id, reason))
}

// translate converte os erros do store nos sentinelas da porta da sala.
func translate(err error) error {
	switch {
	case errors.Is(err, match.ErrMatchClosed):
		return fmt.Errorf("%w: %w", room.ErrMatchClosed, err)
	case errors.Is(err, match.ErrPlyConflict), errors.Is(err, match.ErrPlyGap):
		return fmt.Errorf("%w: %w", room.ErrLogDiverged, err)
	}
	return err
}
