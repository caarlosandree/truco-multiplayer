// Package match persiste partidas como seed + log de ações e deriva resultados do log.
package match

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/db/dbgen"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
)

var (
	ErrNotFound        = errors.New("match: partida não encontrada")
	ErrMatchClosed     = errors.New("match: partida não está em andamento")
	ErrPlyConflict     = errors.New("match: ply já gravado")
	ErrPlyGap          = errors.New("match: ply fora de sequência")
	ErrNotGameOver     = errors.New("match: partida ainda não terminou")
	ErrDuplicatePlayer = errors.New("match: jogador repetido na partida")
)

type Store struct {
	pool *pgxpool.Pool
	q    *dbgen.Queries
	now  func() time.Time
}

func NewStore(pool *pgxpool.Pool, now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{pool: pool, q: dbgen.New(pool), now: now}
}

// notFound converte "sem linhas" e UUID malformado em ErrNotFound.
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) || db.IsInvalidInput(err) {
		return ErrNotFound
	}
	return err
}

func (s *Store) Create(ctx context.Context, seed uint64, firstDealer engine.Seat, players [4]replay.PlayerSeat) (string, error) {
	if firstDealer > 3 {
		return "", fmt.Errorf("match: dealer inicial inválido: %d", firstDealer)
	}
	var id string
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		var err error
		id, err = q.CreateMatch(ctx, dbgen.CreateMatchParams{Seed: int64(seed), FirstDealer: int16(firstDealer),
			EngineVersion: int32(engine.RulesVersion), Now: s.now()})
		if err != nil {
			return err
		}
		for seat, p := range players {
			err := q.AddMatchPlayer(ctx, dbgen.AddMatchPlayerParams{MatchID: id, Seat: int16(seat), UserID: p.UserID, DisplayName: p.DisplayName})
			if db.IsUniqueViolation(err, "match_players_match_id_user_id_key") {
				return ErrDuplicatePlayer
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	return id, err
}

// AppendAction grava uma ação aplicada com sucesso. A sala chama antes de enviar o estado aos jogadores.
// Contrato: a sala é o único escritor das ações da sua partida. Mesmo assim a gravação trava a partida
// em modo compartilhado e exige o ply seguinte, para que nada entre depois de um encerramento confirmado.
func (s *Store) AppendAction(ctx context.Context, id string, ply int, a engine.Action, src replay.Source) error {
	if !src.Valid() {
		return fmt.Errorf("match: origem inválida %q", src)
	}
	if a.Kind == engine.ActSysTimeout {
		return errors.New("match: grave a ação concreta de DefaultAction, não sys_timeout")
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		status, err := q.LockMatchStatus(ctx, id)
		if err != nil {
			return notFound(err)
		}
		if status != string(replay.StatusInProgress) {
			return ErrMatchClosed
		}
		next, err := q.NextPly(ctx, id)
		if err != nil {
			return err
		}
		switch {
		case ply < int(next):
			return ErrPlyConflict
		case ply > int(next):
			return ErrPlyGap
		}
		return q.InsertMatchAction(ctx, dbgen.InsertMatchActionParams{MatchID: id, Ply: int32(ply), Action: raw, Source: string(src), Now: s.now()})
	})
}

func (s *Store) Load(ctx context.Context, id string) (replay.Record, error) {
	m, err := s.q.GetMatch(ctx, id)
	if err != nil {
		return replay.Record{}, notFound(err)
	}
	return loadRecord(ctx, s.q, m)
}

func loadRecord(ctx context.Context, q *dbgen.Queries, m dbgen.Match) (replay.Record, error) {
	rec := replay.Record{ID: m.ID, Seed: uint64(m.Seed), FirstDealer: engine.Seat(m.FirstDealer),
		EngineVersion: int(m.EngineVersion), Status: replay.Status(m.Status)}
	players, err := q.ListMatchPlayers(ctx, m.ID)
	if err != nil {
		return rec, err
	}
	if len(players) != 4 {
		return rec, &replay.CorruptLogError{Ply: -1, Err: fmt.Errorf("partida com %d jogadores", len(players))}
	}
	for _, p := range players {
		rec.Players[p.Seat] = replay.PlayerSeat{UserID: p.UserID, DisplayName: p.DisplayName}
	}
	actions, err := q.ListMatchActions(ctx, m.ID)
	if err != nil {
		return rec, err
	}
	for _, row := range actions {
		var a engine.Action
		if err := json.Unmarshal(row.Action, &a); err != nil {
			return rec, &replay.CorruptLogError{Ply: int(row.Ply), Err: err}
		}
		rec.Actions = append(rec.Actions, replay.LoggedAction{Ply: int(row.Ply), Action: a, Source: replay.Source(row.Source)})
	}
	return rec, nil
}

// Finish encerra a partida a partir do próprio log: reconstrói, confere o fim e grava estatísticas.
func (s *Store) Finish(ctx context.Context, id string) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		m, err := q.GetMatchForUpdate(ctx, id)
		if err != nil {
			return notFound(err)
		}
		if m.Status != string(replay.StatusInProgress) {
			return ErrMatchClosed
		}
		rec, err := loadRecord(ctx, q, m)
		if err != nil {
			return err
		}
		perSeat, err := stats.Derive(rec)
		if errors.Is(err, stats.ErrNotFinished) {
			return ErrNotGameOver
		}
		if err != nil {
			return err
		}
		winner := perSeat[0].Team // assento 0 é sempre da dupla 0
		if !perSeat[0].Won {
			winner = engine.Other(winner)
		}
		now := s.now()
		err = q.FinishMatch(ctx, dbgen.FinishMatchParams{WinnerTeam: int16(winner), ScoreA: int16(perSeat[0].PointsFor),
			ScoreB: int16(perSeat[0].PointsAgainst), Now: now, ID: id})
		if err != nil {
			return err
		}
		return writeStats(ctx, q, id, perSeat, now)
	})
}

func writeStats(ctx context.Context, q *dbgen.Queries, matchID string, perSeat [4]stats.PlayerMatchStats, now time.Time) error {
	for _, p := range perSeat {
		err := q.InsertMatchPlayerStats(ctx, dbgen.InsertMatchPlayerStatsParams{
			MatchID: matchID, UserID: p.UserID, Team: int16(p.Team), Won: p.Won,
			PointsFor: int32(p.PointsFor), PointsAgainst: int32(p.PointsAgainst),
			HandsPlayed: int32(p.HandsPlayed), HandsWon: int32(p.HandsWon),
			RaisesCalled: int32(p.RaisesCalled), RaisesFolded: int32(p.RaisesFolded), RaisesWonByFold: int32(p.RaisesWonByFold),
			ElevenHands: int32(p.ElevenHands), ElevenRuns: int32(p.ElevenRuns), ElevenWon: int32(p.ElevenWon),
			IronHands: int32(p.IronHands), IronWon: int32(p.IronWon), Punished: int32(p.Punished), Timeouts: int32(p.Timeouts),
		})
		if err != nil {
			return err
		}
		var wins int32
		if p.Won {
			wins = 1
		}
		err = q.AddPlayerStats(ctx, dbgen.AddPlayerStatsParams{
			UserID: p.UserID, Wins: wins, Losses: 1 - wins,
			PointsFor: int32(p.PointsFor), PointsAgainst: int32(p.PointsAgainst),
			HandsPlayed: int32(p.HandsPlayed), HandsWon: int32(p.HandsWon),
			RaisesCalled: int32(p.RaisesCalled), RaisesFolded: int32(p.RaisesFolded), RaisesWonByFold: int32(p.RaisesWonByFold),
			ElevenHands: int32(p.ElevenHands), ElevenRuns: int32(p.ElevenRuns), ElevenWon: int32(p.ElevenWon),
			IronHands: int32(p.IronHands), IronWon: int32(p.IronWon), Punished: int32(p.Punished), Timeouts: int32(p.Timeouts),
			Now: now,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Abandon(ctx context.Context, id, reason string) error {
	return AbandonWith(ctx, s.q, id, reason, s.now())
}

// AbandonWith marca a partida como abandonada usando q (que pode estar numa transação).
func AbandonWith(ctx context.Context, q *dbgen.Queries, id, reason string, now time.Time) error {
	n, err := q.AbandonMatch(ctx, dbgen.AbandonMatchParams{Now: now, Reason: reason, ID: id})
	if err != nil {
		return notFound(err)
	}
	if n == 0 {
		if _, err := q.GetMatch(ctx, id); err != nil {
			return notFound(err)
		}
		return ErrMatchClosed
	}
	return nil
}

type PlayerSummary struct {
	Seat        int    `json:"seat"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

type Summary struct {
	ID            string          `json:"id"`
	Status        replay.Status   `json:"status"`
	Players       []PlayerSummary `json:"players"`
	WinnerTeam    *int            `json:"winnerTeam"`
	Score         *[2]int         `json:"score"`
	CreatedAt     time.Time       `json:"createdAt"`
	FinishedAt    *time.Time      `json:"finishedAt"`
	AbandonReason *string         `json:"abandonReason,omitempty"`
}

func (s *Store) Summary(ctx context.Context, id string) (Summary, error) {
	m, err := s.q.GetMatch(ctx, id)
	if err != nil {
		return Summary{}, notFound(err)
	}
	players, err := s.q.ListMatchPlayers(ctx, id)
	if err != nil {
		return Summary{}, err
	}
	out := Summary{ID: m.ID, Status: replay.Status(m.Status), CreatedAt: m.CreatedAt, FinishedAt: m.FinishedAt, AbandonReason: m.AbandonReason}
	for _, p := range players {
		out.Players = append(out.Players, PlayerSummary{Seat: int(p.Seat), Username: p.Username, DisplayName: p.DisplayName})
	}
	if m.WinnerTeam != nil {
		winner := int(*m.WinnerTeam)
		out.WinnerTeam = &winner
	}
	if m.ScoreA != nil && m.ScoreB != nil {
		out.Score = &[2]int{int(*m.ScoreA), int(*m.ScoreB)}
	}
	return out, nil
}
