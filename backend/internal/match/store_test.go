package match_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/enginetest"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testdb"
)

func TestMain(m *testing.M) { testdb.Run(m) }

type env struct {
	store   *match.Store
	pool    *pgxpool.Pool
	clock   *clock.Manual
	players [4]replay.PlayerSeat
}

func newEnv(t *testing.T) env {
	t.Helper()
	pool := testdb.New(t)
	clock := clock.NewManual(time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC))
	e := env{store: match.NewStore(pool, clock.Now), pool: pool, clock: clock}
	for i, name := range []string{"ana", "bia", "caio", "duda"} {
		e.players[i] = replay.PlayerSeat{UserID: testdb.CreateUser(t, pool, name), DisplayName: name}
	}
	return e
}

// persist grava os primeiros n passos (n < 0 = todos) de uma partida aleatória.
func (e env) persist(t *testing.T, seed uint64, n int) (string, replay.Record) {
	t.Helper()
	ctx := context.Background()
	rec, _ := enginetest.RandomRecord(seed)
	rec.Players = e.players
	if n >= 0 {
		rec.Actions = rec.Actions[:n]
	}
	id, err := e.store.Create(ctx, rec.Seed, rec.FirstDealer, e.players)
	if err != nil {
		t.Fatal(err)
	}
	for _, logged := range rec.Actions {
		if err := e.store.AppendAction(ctx, id, logged.Ply, logged.Action, logged.Source); err != nil {
			t.Fatalf("ply %d: %v", logged.Ply, err)
		}
	}
	rec.ID = id
	return id, rec
}

func TestCreateAndLoad(t *testing.T) {
	e := newEnv(t)
	id, want := e.persist(t, 1, 10)
	got, err := e.store.Load(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id || got.Seed != want.Seed || got.FirstDealer != want.FirstDealer || got.EngineVersion != engine.RulesVersion ||
		got.Status != replay.StatusInProgress || got.Players != e.players || !reflect.DeepEqual(got.Actions, want.Actions) {
		t.Fatalf("Load divergente:\n got %+v\nwant %+v", got, want)
	}
}

func TestSeedUsesFullUint64Range(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	const seed = uint64(1<<64 - 2)
	id, err := e.store.Create(ctx, seed, 3, e.players)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := e.store.Load(ctx, id)
	if got.Seed != seed {
		t.Fatalf("seed = %d", got.Seed)
	}
}

func TestCreateRejectsDuplicatePlayer(t *testing.T) {
	e := newEnv(t)
	players := e.players
	players[3] = players[0]
	if _, err := e.store.Create(context.Background(), 1, 0, players); !errors.Is(err, match.ErrDuplicatePlayer) {
		t.Fatalf("err = %v", err)
	}
}

func TestAppendActionErrors(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id, rec := e.persist(t, 1, 3)
	first := rec.Actions[0]
	if err := e.store.AppendAction(ctx, id, 0, first.Action, first.Source); !errors.Is(err, match.ErrPlyConflict) {
		t.Fatalf("ply repetido: %v", err)
	}
	for _, missing := range []string{"00000000-0000-7000-8000-000000000000", "nao-e-uuid"} {
		if err := e.store.AppendAction(ctx, missing, 0, first.Action, first.Source); !errors.Is(err, match.ErrNotFound) {
			t.Fatalf("partida %q: %v", missing, err)
		}
	}
	if err := e.store.AppendAction(ctx, id, 3, engine.Action{Kind: engine.ActSysTimeout, Seat: 1}, replay.SourceTimeout); err == nil {
		t.Fatal("sys_timeout não pode ser gravado")
	}
	if err := e.store.AppendAction(ctx, id, 3, first.Action, replay.Source("bot")); err == nil {
		t.Fatal("origem inválida aceita")
	}
	if err := e.store.Abandon(ctx, id, "teste"); err != nil {
		t.Fatal(err)
	}
	if err := e.store.AppendAction(ctx, id, 3, first.Action, first.Source); !errors.Is(err, match.ErrMatchClosed) {
		t.Fatalf("após abandono: %v", err)
	}
}

func TestConcurrentAppendOfSamePlyHasSingleWinner(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id, err := e.store.Create(ctx, 1, 0, e.players)
	if err != nil {
		t.Fatal(err)
	}
	start := engine.Action{Kind: engine.ActSysStartHand}
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = e.store.AppendAction(ctx, id, 0, start, replay.SourceSystem)
		}()
	}
	wg.Wait()
	ok, conflicts := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, match.ErrPlyConflict):
			conflicts++
		default:
			t.Fatalf("erro inesperado: %v", err)
		}
	}
	if ok != 1 || conflicts != 7 {
		t.Fatalf("ok=%d conflitos=%d", ok, conflicts)
	}
}

func TestFinishWritesResultAndStats(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id, rec := e.persist(t, 5, -1)
	if err := e.store.Finish(ctx, id); err != nil {
		t.Fatal(err)
	}
	final, _ := replay.Rebuild(rec)
	summary, err := e.store.Summary(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Status != replay.StatusFinished || summary.WinnerTeam == nil || *summary.WinnerTeam != int(*final.Winner) ||
		summary.Score == nil || *summary.Score != final.Score || summary.FinishedAt == nil {
		t.Fatalf("summary = %+v, placar ao vivo %v", summary, final.Score)
	}
	derived, _ := stats.Derive(rec)
	for _, p := range derived {
		var played, wins, pointsFor, hands int
		err := e.pool.QueryRow(ctx, `SELECT matches_played, wins, points_for, hands_played FROM player_stats WHERE user_id = $1`, p.UserID).
			Scan(&played, &wins, &pointsFor, &hands)
		if err != nil {
			t.Fatal(err)
		}
		wantWins := 0
		if p.Won {
			wantWins = 1
		}
		if played != 1 || wins != wantWins || pointsFor != p.PointsFor || hands != p.HandsPlayed {
			t.Fatalf("player_stats de %s: %d %d %d %d", p.UserID, played, wins, pointsFor, hands)
		}
	}
	var rows int
	_ = e.pool.QueryRow(ctx, "SELECT count(*) FROM match_player_stats WHERE match_id = $1", id).Scan(&rows)
	if rows != 4 {
		t.Fatalf("%d linhas em match_player_stats", rows)
	}
	if err := e.store.Finish(ctx, id); !errors.Is(err, match.ErrMatchClosed) {
		t.Fatalf("Finish duplicado: %v", err)
	}
}

func TestFinishRejectsUnfinishedMatch(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id, _ := e.persist(t, 5, 20)
	if err := e.store.Finish(ctx, id); !errors.Is(err, match.ErrNotGameOver) {
		t.Fatalf("err = %v", err)
	}
	summary, _ := e.store.Summary(ctx, id)
	if summary.Status != replay.StatusInProgress {
		t.Fatalf("status = %s", summary.Status)
	}
}

func TestPlayerStatsAccumulateAcrossMatches(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for _, seed := range []uint64{11, 12} {
		id, _ := e.persist(t, seed, -1)
		if err := e.store.Finish(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range e.players {
		var played, wins, losses int
		_ = e.pool.QueryRow(ctx, `SELECT matches_played, wins, losses FROM player_stats WHERE user_id = $1`, p.UserID).Scan(&played, &wins, &losses)
		if played != 2 || wins+losses != 2 {
			t.Fatalf("%s: jogou %d, %d vitórias, %d derrotas", p.DisplayName, played, wins, losses)
		}
	}
}

func TestAbandon(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id, _ := e.persist(t, 1, 5)
	if err := e.store.Abandon(ctx, id, "sala vazia"); err != nil {
		t.Fatal(err)
	}
	summary, _ := e.store.Summary(ctx, id)
	if summary.Status != replay.StatusAbandoned || summary.AbandonReason == nil || *summary.AbandonReason != "sala vazia" || summary.FinishedAt == nil {
		t.Fatalf("summary = %+v", summary)
	}
	if err := e.store.Abandon(ctx, id, "de novo"); !errors.Is(err, match.ErrMatchClosed) {
		t.Fatalf("abandono duplicado: %v", err)
	}
	if err := e.store.Finish(ctx, id); !errors.Is(err, match.ErrMatchClosed) {
		t.Fatalf("Finish de abandonada: %v", err)
	}
	if err := e.store.Abandon(ctx, "00000000-0000-7000-8000-000000000000", "x"); !errors.Is(err, match.ErrNotFound) {
		t.Fatalf("inexistente: %v", err)
	}
}

func TestSummaryListsPlayers(t *testing.T) {
	e := newEnv(t)
	id, _ := e.persist(t, 1, 0)
	summary, err := e.store.Summary(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Players) != 4 || summary.Players[2].Username != "caio" || summary.Players[2].Seat != 2 || summary.WinnerTeam != nil {
		t.Fatalf("summary = %+v", summary)
	}
	if _, err := e.store.Summary(context.Background(), "nao-e-uuid"); !errors.Is(err, match.ErrNotFound) {
		t.Fatalf("id inválido: %v", err)
	}
}

func TestRecover(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	partialID, partial := e.persist(t, 21, 30)

	corruptID, _ := e.persist(t, 22, 1)
	// ply 1 ilegal: ninguém pode jogar carta durante o corte.
	if err := e.store.AppendAction(ctx, corruptID, 1, engine.Action{Kind: engine.ActPlayCard, Seat: 0}, replay.SourcePlayer); err != nil {
		t.Fatal(err)
	}

	doneID, _ := e.persist(t, 23, -1) // terminou, mas o servidor caiu antes do Finish

	oldID, _ := e.persist(t, 24, 3)
	if _, err := e.pool.Exec(ctx, "UPDATE matches SET engine_version = 999 WHERE id = $1", oldID); err != nil {
		t.Fatal(err)
	}

	recovered, err := e.store.Recover(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) != 1 || recovered[0].Record.ID != partialID || recovered[0].NextPly != 30 {
		t.Fatalf("recuperadas = %+v", recovered)
	}
	want, _ := replay.Rebuild(partial)
	if !reflect.DeepEqual(recovered[0].State, want) {
		t.Fatal("estado recuperado diverge do log")
	}
	status := func(id string) (string, *string) {
		var s string
		var reason *string
		_ = e.pool.QueryRow(ctx, "SELECT status, abandon_reason FROM matches WHERE id = $1", id).Scan(&s, &reason)
		return s, reason
	}
	if s, r := status(corruptID); s != "abandoned" || r == nil || *r != "corrupt_log" {
		t.Fatalf("corrompida: %s %v", s, r)
	}
	if s, _ := status(doneID); s != "finished" {
		t.Fatalf("terminada sem Finish: %s", s)
	}
	if s, r := status(oldID); s != "abandoned" || r == nil || *r != "incompatible_version" {
		t.Fatalf("versão antiga: %s %v", s, r)
	}
}

func TestRecomputeStatsRestoresAggregates(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for _, seed := range []uint64{31, 32} {
		id, _ := e.persist(t, seed, -1)
		if err := e.store.Finish(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := func() map[string][3]int {
		out := map[string][3]int{}
		rows, _ := e.pool.Query(ctx, "SELECT user_id::text, matches_played, wins, points_for FROM player_stats")
		defer rows.Close()
		for rows.Next() {
			var id string
			var v [3]int
			_ = rows.Scan(&id, &v[0], &v[1], &v[2])
			out[id] = v
		}
		return out
	}
	before := snapshot()
	if _, err := e.pool.Exec(ctx, "UPDATE player_stats SET wins = 999"); err != nil {
		t.Fatal(err)
	}
	skipped, err := e.store.RecomputeStats(ctx)
	if err != nil || len(skipped) != 0 {
		t.Fatalf("skipped=%v err=%v", skipped, err)
	}
	if after := snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatalf("recálculo divergente:\nantes %v\ndepois %v", before, after)
	}
}

func TestRecomputeStatsSkipsCorruptFinishedMatch(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	goodID, _ := e.persist(t, 41, -1)
	if err := e.store.Finish(ctx, goodID); err != nil {
		t.Fatal(err)
	}
	badID, _ := e.persist(t, 42, -1)
	if err := e.store.Finish(ctx, badID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(ctx, "UPDATE match_actions SET action = '\"lixo\"'::jsonb WHERE match_id = $1 AND ply = 0", badID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(ctx, "UPDATE player_stats SET wins = 999"); err != nil {
		t.Fatal(err)
	}
	skipped, err := e.store.RecomputeStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(skipped, []string{badID}) {
		t.Fatalf("skipped = %v, want [%s]", skipped, badID)
	}
	var rows int
	_ = e.pool.QueryRow(ctx, "SELECT count(*) FROM match_player_stats WHERE match_id = $1", goodID).Scan(&rows)
	var bad int
	_ = e.pool.QueryRow(ctx, "SELECT count(*) FROM match_player_stats WHERE match_id = $1", badID).Scan(&bad)
	var wins999 int
	_ = e.pool.QueryRow(ctx, "SELECT count(*) FROM player_stats WHERE wins = 999").Scan(&wins999)
	if rows != 4 || bad != 0 || wins999 != 0 {
		t.Fatalf("good=%d bad=%d wins999=%d", rows, bad, wins999)
	}
}
