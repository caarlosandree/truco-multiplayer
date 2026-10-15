package stats_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/enginetest"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testdb"
)

func TestMain(m *testing.M) { testdb.Run(m) }

type world struct {
	pool    *pgxpool.Pool
	store   *match.Store
	clock   *clock.Manual
	players [4]replay.PlayerSeat
}

func newWorld(t *testing.T) world {
	t.Helper()
	pool := testdb.New(t)
	clock := clock.NewManual(time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC))
	w := world{pool: pool, store: match.NewStore(pool, clock.Now), clock: clock}
	for i, name := range []string{"ana", "bia", "caio", "duda"} {
		w.players[i] = replay.PlayerSeat{UserID: testdb.CreateUser(t, pool, name), DisplayName: name}
	}
	return w
}

// play grava e finaliza uma partida completa; complete=false deixa em andamento.
func (w world) play(t *testing.T, seed uint64, complete bool) string {
	t.Helper()
	ctx := context.Background()
	rec, _ := enginetest.RandomRecord(seed)
	id, err := w.store.Create(ctx, rec.Seed, rec.FirstDealer, w.players)
	if err != nil {
		t.Fatal(err)
	}
	actions := rec.Actions
	if !complete {
		actions = actions[:5]
	}
	for _, a := range actions {
		if err := w.store.AppendAction(ctx, id, a.Ply, a.Action, a.Source); err != nil {
			t.Fatal(err)
		}
	}
	w.clock.Advance(time.Minute)
	if complete {
		if err := w.store.Finish(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func TestRankingOrderAndMinimum(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.play(t, 1, true)
	w.play(t, 2, true)
	entries, err := stats.NewReader(w.pool, 1).Ranking(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("%d entradas", len(entries))
	}
	for i, e := range entries {
		if e.Position != i+1 || e.MatchesPlayed != 2 || e.Wins+e.Losses != 2 || e.WinRate != float64(e.Wins)/2 {
			t.Fatalf("entrada %d = %+v", i, e)
		}
		if i > 0 {
			prev := entries[i-1]
			if prev.Wins < e.Wins || (prev.Wins == e.Wins && prev.Username > e.Username) {
				t.Fatalf("ordem errada: %+v antes de %+v", prev, e)
			}
		}
	}
	if entries, _ := stats.NewReader(w.pool, 3).Ranking(ctx, 0); len(entries) != 0 {
		t.Fatalf("mínimo de partidas ignorado: %+v", entries)
	}
	_, _ = w.pool.Exec(ctx, "UPDATE users SET disabled_at = now() WHERE username = 'ana'")
	if entries, _ := stats.NewReader(w.pool, 1).Ranking(ctx, 0); len(entries) != 3 {
		t.Fatal("conta desativada não aparece no ranking")
	}
}

func TestProfile(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	reader := stats.NewReader(w.pool, 1)
	p, err := reader.Profile(ctx, "ana")
	if err != nil || p.Username != "ana" || p.Stats.MatchesPlayed != 0 || p.Stats.WinRate != 0 {
		t.Fatalf("perfil sem partidas: %+v %v", p, err)
	}
	w.play(t, 3, true)
	p, _ = reader.Profile(ctx, "ANA")
	if p.Stats.MatchesPlayed != 1 || p.Stats.HandsPlayed == 0 {
		t.Fatalf("perfil após partida: %+v", p)
	}
	if _, err := reader.Profile(ctx, "ninguem"); !errors.Is(err, stats.ErrUserNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestHistoryPagination(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	first := w.play(t, 4, true)
	w.play(t, 5, true)
	last := w.play(t, 6, true)
	w.play(t, 7, false) // em andamento: fora do histórico
	reader := stats.NewReader(w.pool, 1)
	page, err := reader.History(ctx, "bia", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.Items[0].ID != last || page.NextCursor == "" {
		t.Fatalf("página 1 = %+v", page)
	}
	if page.Items[0].Team != 1 {
		t.Fatalf("bia senta no assento 1 (dupla 1): %+v", page.Items[0])
	}
	summary, _ := w.store.Summary(ctx, last)
	if page.Items[0].Won != (*summary.WinnerTeam == 1) || *page.Items[0].Score != *summary.Score {
		t.Fatalf("item %+v vs resumo %+v", page.Items[0], summary)
	}
	page2, err := reader.History(ctx, "bia", page.NextCursor, 2)
	if err != nil || len(page2.Items) != 1 || page2.Items[0].ID != first || page2.NextCursor != "" {
		t.Fatalf("página 2 = %+v err %v", page2, err)
	}
	if _, err := reader.History(ctx, "bia", "lixo!", 2); err == nil {
		t.Fatal("cursor inválido aceito")
	}
	if _, err := reader.History(ctx, "ninguem", "", 2); !errors.Is(err, stats.ErrUserNotFound) {
		t.Fatalf("err = %v", err)
	}
}
