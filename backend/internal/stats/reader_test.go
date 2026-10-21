package stats_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/db/dbgen"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/enginetest"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/pagination"
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

// playWith grava e finaliza uma partida completa com os jogadores dados.
func (w world) playWith(t *testing.T, seed uint64, players [4]replay.PlayerSeat) string {
	t.Helper()
	ctx := context.Background()
	rec, _ := enginetest.RandomRecord(seed)
	id, err := w.store.Create(ctx, rec.Seed, rec.FirstDealer, players)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range rec.Actions {
		if err := w.store.AppendAction(ctx, id, a.Ply, a.Action, a.Source); err != nil {
			t.Fatal(err)
		}
	}
	w.clock.Advance(time.Minute)
	if err := w.store.Finish(ctx, id); err != nil {
		t.Fatal(err)
	}
	return id
}

// playForfeit grava 20 ações e encerra por W.O. da dupla 1.
func (w world) playForfeit(t *testing.T, seed uint64) string {
	t.Helper()
	ctx := context.Background()
	rec, _ := enginetest.RandomRecord(seed)
	id, err := w.store.Create(ctx, rec.Seed, rec.FirstDealer, w.players)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range rec.Actions[:20] {
		if err := w.store.AppendAction(ctx, id, a.Ply, a.Action, a.Source); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.store.AppendAction(ctx, id, 20, engine.Action{Kind: engine.ActSysForfeit, Team: 1}, replay.SourceSystem); err != nil {
		t.Fatal(err)
	}
	w.clock.Advance(time.Minute)
	if err := w.store.Finish(ctx, id); err != nil {
		t.Fatal(err)
	}
	return id
}

// play grava e finaliza uma partida completa; complete=false deixa em andamento.
func (w world) play(t *testing.T, seed uint64, complete bool) string {
	t.Helper()
	if complete {
		return w.playWith(t, seed, w.players)
	}
	ctx := context.Background()
	rec, _ := enginetest.RandomRecord(seed)
	id, err := w.store.Create(ctx, rec.Seed, rec.FirstDealer, w.players)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range rec.Actions[:5] {
		if err := w.store.AppendAction(ctx, id, a.Ply, a.Action, a.Source); err != nil {
			t.Fatal(err)
		}
	}
	w.clock.Advance(time.Minute)
	return id
}

func (w world) reader(minMatches int) *stats.Reader {
	return stats.NewReader(w.pool, minMatches, w.clock.Now)
}

func rank(t *testing.T, r *stats.Reader, q stats.RankingQuery) stats.RankingResult {
	t.Helper()
	res, err := r.Ranking(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func usernames(items []stats.RankingEntry) []string {
	out := make([]string, len(items))
	for i, e := range items {
		out[i] = e.Username
	}
	return out
}

func TestRankingOrderAndMinimum(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.play(t, 1, true)
	w.play(t, 2, true)
	res := rank(t, w.reader(1), stats.RankingQuery{})
	if len(res.Items) != 4 || res.MinMatches != 1 {
		t.Fatalf("resultado = %+v", res)
	}
	for i, e := range res.Items {
		if e.Position != i+1 || e.MatchesPlayed != 2 || e.Wins+e.Losses != 2 || e.WinRate != float64(e.Wins)/2 {
			t.Fatalf("entrada %d = %+v", i, e)
		}
		if i > 0 {
			prev := res.Items[i-1]
			if prev.Wins < e.Wins || (prev.Wins == e.Wins && prev.Username > e.Username) {
				t.Fatalf("ordem errada: %+v antes de %+v", prev, e)
			}
		}
	}
	if res := rank(t, w.reader(3), stats.RankingQuery{Metric: stats.MetricWinRate}); len(res.Items) != 0 {
		t.Fatalf("mínimo de partidas ignorado: %+v", res.Items)
	}
	_, _ = w.pool.Exec(ctx, "UPDATE users SET disabled_at = now() WHERE username = 'ana'")
	if res := rank(t, w.reader(1), stats.RankingQuery{}); len(res.Items) != 3 {
		t.Fatal("conta desativada não aparece no ranking")
	}
}

func TestRankingInvalidQuery(t *testing.T) {
	w := newWorld(t)
	for _, q := range []stats.RankingQuery{{Period: "ano"}, {Metric: "pontos"}} {
		if _, err := w.reader(1).Ranking(context.Background(), q); !errors.Is(err, stats.ErrInvalidRankingQuery) {
			t.Fatalf("%+v: err = %v", q, err)
		}
	}
}

func TestRankingWeekBoundary(t *testing.T) {
	w := newWorld(t)
	// Domingo 23:30 em Brasília (segunda 02:30 UTC) e segunda 00:00 em Brasília (03:00 UTC).
	w.clock.Advance(time.Date(2026, 10, 19, 2, 29, 0, 0, time.UTC).Sub(w.clock.Now()))
	w.play(t, 1, true)
	w.clock.Advance(29 * time.Minute)
	w.play(t, 2, true)
	w.clock.Advance(time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC).Sub(w.clock.Now()))
	r := w.reader(1)
	week := rank(t, r, stats.RankingQuery{Period: stats.PeriodWeek})
	month := rank(t, r, stats.RankingQuery{Period: stats.PeriodMonth})
	all := rank(t, r, stats.RankingQuery{Period: stats.PeriodAll})
	if len(week.Items) != 4 || week.Items[0].MatchesPlayed != 1 {
		t.Fatalf("semana deveria contar só a partida de segunda: %+v", week.Items)
	}
	if month.Items[0].MatchesPlayed != 2 || all.Items[0].MatchesPlayed != 2 {
		t.Fatalf("mês/geral deveriam contar as duas: %+v %+v", month.Items, all.Items)
	}
}

func TestRankingWinsHasNoMinimumWinrateHas(t *testing.T) {
	w := newWorld(t)
	w.play(t, 1, true)
	w.play(t, 2, true)
	edu := replay.PlayerSeat{UserID: testdb.CreateUser(t, w.pool, "edu"), DisplayName: "edu"}
	w.playWith(t, 3, [4]replay.PlayerSeat{w.players[0], w.players[1], w.players[2], edu})
	r := w.reader(3)

	wins := rank(t, r, stats.RankingQuery{Metric: stats.MetricWins})
	if len(wins.Items) != 5 || wins.MinMatches != 3 {
		t.Fatalf("wins = %+v", wins)
	}
	for i := 1; i < len(wins.Items); i++ {
		a, b := wins.Items[i-1], wins.Items[i]
		if !(a.Wins > b.Wins ||
			a.Wins == b.Wins && (a.WinRate > b.WinRate ||
				a.WinRate == b.WinRate && (a.MatchesPlayed > b.MatchesPlayed ||
					a.MatchesPlayed == b.MatchesPlayed && a.Username < b.Username))) {
			t.Fatalf("ordem de wins errada: %+v antes de %+v", a, b)
		}
	}
	rate := rank(t, r, stats.RankingQuery{Metric: stats.MetricWinRate})
	if len(rate.Items) != 3 || rate.MinMatches != 3 {
		t.Fatalf("winrate deveria excluir quem tem menos de 3 partidas: %+v", usernames(rate.Items))
	}
	for _, e := range rate.Items {
		if e.Username == "duda" || e.Username == "edu" {
			t.Fatalf("%s não atinge o mínimo", e.Username)
		}
	}
	for i := 1; i < len(rate.Items); i++ {
		a, b := rate.Items[i-1], rate.Items[i]
		if !(a.WinRate > b.WinRate ||
			a.WinRate == b.WinRate && (a.Wins > b.Wins ||
				a.Wins == b.Wins && (a.MatchesPlayed > b.MatchesPlayed ||
					a.MatchesPlayed == b.MatchesPlayed && a.Username < b.Username))) {
			t.Fatalf("ordem de winrate errada: %+v antes de %+v", a, b)
		}
	}
}

func TestRankingMeOutsideTop(t *testing.T) {
	w := newWorld(t)
	w.play(t, 1, true)
	r := w.reader(1)
	full := rank(t, r, stats.RankingQuery{})
	fourth := full.Items[3]
	var fourthID string
	for _, p := range w.players {
		if p.DisplayName == fourth.Username {
			fourthID = p.UserID
		}
	}
	res := rank(t, r, stats.RankingQuery{Limit: 2, ViewerID: fourthID})
	if len(res.Items) != 2 || res.Me == nil || res.Me.Position != 4 || res.Me.Username != fourth.Username {
		t.Fatalf("res = %+v me=%+v", res.Items, res.Me)
	}
	topID := ""
	for _, p := range w.players {
		if p.DisplayName == full.Items[0].Username {
			topID = p.UserID
		}
	}
	inTop := rank(t, r, stats.RankingQuery{Limit: 2, ViewerID: topID})
	if inTop.Me == nil || len(inTop.Items) != 2 || inTop.Me.Position != 1 || inTop.Me.Username != full.Items[0].Username {
		t.Fatalf("me deve vir preenchido mesmo dentro do top: %+v", inTop)
	}
	onlyMe := rank(t, r, stats.RankingQuery{ViewerID: fourthID, MeOnly: true})
	if len(onlyMe.Items) != 0 || onlyMe.Me == nil || onlyMe.Me.Position != 4 {
		t.Fatalf("MeOnly = %+v", onlyMe)
	}
	stranger := testdb.CreateUser(t, w.pool, "zeca")
	if res := rank(t, r, stats.RankingQuery{ViewerID: stranger}); res.Me != nil {
		t.Fatalf("viewer sem partidas: me = %+v", res.Me)
	}
}

func TestRankingIgnoresAbandonedAndDisabled(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	aborted := w.play(t, 7, false)
	if err := match.AbandonWith(ctx, dbgen.New(w.pool), aborted, "abortada pelo administrador", replay.EndAborted, w.clock.Now()); err != nil {
		t.Fatal(err)
	}
	r := w.reader(1)
	for _, p := range []stats.Period{stats.PeriodAll, stats.PeriodMonth, stats.PeriodWeek} {
		if res := rank(t, r, stats.RankingQuery{Period: p}); len(res.Items) != 0 {
			t.Fatalf("%s: partida abortada contou: %+v", p, res.Items)
		}
	}
	w.play(t, 1, true)
	_, _ = w.pool.Exec(ctx, "UPDATE users SET disabled_at = now() WHERE username = 'ana'")
	for _, p := range []stats.Period{stats.PeriodAll, stats.PeriodMonth, stats.PeriodWeek} {
		res := rank(t, r, stats.RankingQuery{Period: p, ViewerID: w.players[0].UserID})
		if len(res.Items) != 3 || res.Me != nil {
			t.Fatalf("%s: desativado apareceu: %+v me=%+v", p, res.Items, res.Me)
		}
	}
}

func TestProfilePositions(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.play(t, 1, true)
	w.clock.Advance(30 * 24 * time.Hour)
	r := w.reader(3)
	p, err := r.Profile(ctx, "ana", false)
	if err != nil {
		t.Fatal(err)
	}
	if p.Positions.Week.WinRate != nil || p.Positions.Week.Wins != nil {
		t.Fatalf("sem partidas na semana: %+v", p.Positions.Week)
	}
	if p.Positions.All.WinRate != nil {
		t.Fatalf("1 partida < mínimo 3, aproveitamento deveria ser nulo: %+v", p.Positions.All)
	}
	all := rank(t, r, stats.RankingQuery{ViewerID: w.players[0].UserID})
	if all.Me == nil || p.Positions.All.Wins == nil || *p.Positions.All.Wins != all.Me.Position {
		t.Fatalf("posição geral = %+v, ranking = %+v", p.Positions.All, all.Me)
	}
	if p.Positions.Month.Wins != nil {
		t.Fatalf("a partida é de um mês atrás: %+v", p.Positions.Month)
	}
}

func TestProfilePositionsWithMinimumReached(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.play(t, 1, true)
	w.play(t, 2, true)
	r := w.reader(2)
	for i, pl := range w.players {
		p, err := r.Profile(ctx, pl.DisplayName, false)
		if err != nil {
			t.Fatal(err)
		}
		for name, c := range map[stats.Period]stats.PositionPair{stats.PeriodAll: p.Positions.All, stats.PeriodMonth: p.Positions.Month, stats.PeriodWeek: p.Positions.Week} {
			for _, m := range []stats.Metric{stats.MetricWins, stats.MetricWinRate} {
				me := rank(t, r, stats.RankingQuery{Period: name, Metric: m, ViewerID: pl.UserID}).Me
				got := c.Wins
				if m == stats.MetricWinRate {
					got = c.WinRate
				}
				if me == nil || got == nil || *got != me.Position {
					t.Fatalf("jogador %d %s/%s: posição %v, ranking %+v", i, name, m, got, me)
				}
			}
		}
	}
}

func TestProfile(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	reader := w.reader(1)
	p, err := reader.Profile(ctx, "ana", false)
	if err != nil || p.Username != "ana" || p.Stats.MatchesPlayed != 0 || p.Stats.WinRate != 0 {
		t.Fatalf("perfil sem partidas: %+v %v", p, err)
	}
	w.play(t, 3, true)
	p, _ = reader.Profile(ctx, "ANA", false)
	if p.Stats.MatchesPlayed != 1 || p.Stats.HandsPlayed == 0 {
		t.Fatalf("perfil após partida: %+v", p)
	}
	if _, err := reader.Profile(ctx, "ninguem", false); !errors.Is(err, stats.ErrUserNotFound) {
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
	reader := w.reader(1)
	page, err := reader.History(ctx, "bia", "", 2, false)
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
	page2, err := reader.History(ctx, "bia", page.NextCursor, 2, false)
	if err != nil || len(page2.Items) != 1 || page2.Items[0].ID != first || page2.NextCursor != "" {
		t.Fatalf("página 2 = %+v err %v", page2, err)
	}
	if _, err := reader.History(ctx, "bia", "lixo!", 2, false); err == nil {
		t.Fatal("cursor inválido aceito")
	}
	if _, err := reader.History(ctx, "ninguem", "", 2, false); !errors.Is(err, stats.ErrUserNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestHistoryHasPlayersAndEndReason(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	normal := w.play(t, 4, true)
	wo := w.playForfeit(t, 6)
	aborted := w.play(t, 7, false)
	if err := match.AbandonWith(ctx, dbgen.New(w.pool), aborted, "abortada pelo administrador", replay.EndAborted, w.clock.Now()); err != nil {
		t.Fatal(err)
	}
	page, err := w.reader(1).History(ctx, "bia", "", 10, false)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]stats.HistoryItem{}
	for _, it := range page.Items {
		got[it.ID] = it
		if len(it.Players) != 4 {
			t.Fatalf("%d jogadores", len(it.Players))
		}
		for seat, p := range it.Players {
			if p.Seat != seat || p.Username != w.players[seat].DisplayName || p.DisplayName != w.players[seat].DisplayName {
				t.Fatalf("jogadores fora de ordem: %+v", it.Players)
			}
		}
	}
	if len(got) != 3 {
		t.Fatalf("%d itens", len(got))
	}
	if got[normal].EndReason != replay.EndGameOver || got[wo].EndReason != replay.EndForfeit || got[aborted].EndReason != replay.EndAborted {
		t.Fatalf("motivos: %v %v %v", got[normal].EndReason, got[wo].EndReason, got[aborted].EndReason)
	}
	if got[wo].WinnerTeam == nil || *got[wo].WinnerTeam != 0 {
		t.Fatalf("W.O. da dupla 1: %+v", got[wo])
	}
	if a := got[aborted]; a.Won || a.WinnerTeam != nil {
		t.Fatalf("abortada: %+v", a)
	}
}

func TestProfileRecentMatchesAndPartners(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	eva := replay.PlayerSeat{UserID: testdb.CreateUser(t, w.pool, "eva"), DisplayName: "eva"}
	other := w.players
	other[2] = eva
	var ids []string
	for i := range 6 {
		if i < 4 {
			ids = append(ids, w.playWith(t, uint64(10+i), w.players))
		} else {
			ids = append(ids, w.playWith(t, uint64(10+i), other))
		}
	}
	reader := w.reader(1)
	p, err := reader.Profile(ctx, "ana", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.RecentMatches) != 5 || p.RecentMatches[0].ID != ids[5] || p.RecentMatches[4].ID != ids[1] {
		t.Fatalf("recentes: %+v", p.RecentMatches)
	}
	var wins int
	err = w.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE won) FROM match_player_stats s
		JOIN match_players me ON me.match_id = s.match_id AND me.user_id = s.user_id
		JOIN users u ON u.id = me.user_id AND u.username = 'ana'
		JOIN match_players p ON p.match_id = me.match_id AND p.seat = 2
		JOIN users pu ON pu.id = p.user_id AND pu.username = 'caio'`).Scan(&wins)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Partners) != 2 || p.Partners[0].Username != "caio" || p.Partners[0].Matches != 4 || p.Partners[0].Wins != wins ||
		p.Partners[0].WinRate != float64(wins)/4 || p.Partners[1].Username != "eva" || p.Partners[1].Matches != 2 {
		t.Fatalf("parceiros: %+v (vitórias com caio: %d)", p.Partners, wins)
	}
	_, _ = w.pool.Exec(ctx, "UPDATE users SET disabled_at = now() WHERE username = 'caio'")
	p, _ = reader.Profile(ctx, "ana", false)
	if len(p.Partners) != 1 || p.Partners[0].Username != "eva" {
		t.Fatalf("desativado não deve aparecer: %+v", p.Partners)
	}
	empty, _ := reader.Profile(ctx, "duda", false)
	if empty.RecentMatches == nil || empty.Partners == nil {
		t.Fatal("slices não podem ser nil")
	}
	fresh, _ := reader.Profile(ctx, "bia", false)
	if len(fresh.Partners) != 1 || fresh.Partners[0].Username != "duda" {
		t.Fatalf("parceiro da bia: %+v", fresh.Partners)
	}
}

func TestDisabledProfileIsNotFoundForNonAdmin(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.play(t, 3, true)
	_, _ = w.pool.Exec(ctx, "UPDATE users SET disabled_at = now() WHERE username = 'duda'")
	reader := w.reader(1)
	if _, err := reader.Profile(ctx, "duda", false); !errors.Is(err, stats.ErrUserNotFound) {
		t.Fatalf("Profile err = %v", err)
	}
	if _, err := reader.History(ctx, "duda", "", 5, false); !errors.Is(err, stats.ErrUserNotFound) {
		t.Fatalf("History err = %v", err)
	}
	if p, err := reader.Profile(ctx, "duda", true); err != nil || p.Username != "duda" {
		t.Fatalf("admin: %+v %v", p, err)
	}
	if _, err := reader.History(ctx, "duda", "", 5, true); err != nil {
		t.Fatalf("admin History: %v", err)
	}
}

func TestDirectorySearchHidesEmailAndDisabled(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	mk := func(username, display string, verified bool) {
		testdb.CreateUser(t, w.pool, username)
		_, err := w.pool.Exec(ctx, "UPDATE users SET display_name = $2, email_verified_at = CASE WHEN $3 THEN now() END WHERE username = $1", username, display, verified)
		if err != nil {
			t.Fatal(err)
		}
	}
	mk("fulano", "Fulana da Silva", true)
	mk("anonimo", "Anônimo", false)
	reader := w.reader(1)
	page, err := reader.Directory(ctx, "an", "")
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, e := range page.Items {
		names = append(names, e.Username)
	}
	if fmt.Sprint(names) != "[ana fulano]" {
		t.Fatalf("busca 'an' = %v", names)
	}
	page, _ = reader.Directory(ctx, "FULANA", "")
	if len(page.Items) != 1 || page.Items[0].Username != "fulano" {
		t.Fatalf("busca por nome: %+v", page.Items)
	}
	_, _ = w.pool.Exec(ctx, "UPDATE users SET disabled_at = now() WHERE username = 'ana'")
	if page, _ = reader.Directory(ctx, "an", ""); len(page.Items) != 1 {
		t.Fatalf("desativado apareceu: %+v", page.Items)
	}
	raw, _ := json.Marshal(page.Items[0])
	var keys map[string]any
	_ = json.Unmarshal(raw, &keys)
	if len(keys) != 2 || keys["username"] == nil || keys["displayName"] == nil {
		t.Fatalf("chaves: %s", raw)
	}
	if page, _ = reader.Directory(ctx, "%", ""); len(page.Items) != 0 {
		t.Fatalf("curinga não escapado: %+v", page.Items)
	}
}

func TestDirectoryPagination(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	for i := range 21 { // + 4 do world = 25
		testdb.CreateUser(t, w.pool, fmt.Sprintf("u%02d", i))
	}
	_, _ = w.pool.Exec(ctx, "UPDATE users SET email_verified_at = now() WHERE email_verified_at IS NULL")
	reader := w.reader(1)
	p1, err := reader.Directory(ctx, "", "")
	if err != nil || len(p1.Items) != 20 || p1.NextCursor == "" {
		t.Fatalf("p1 = %d %q %v", len(p1.Items), p1.NextCursor, err)
	}
	p2, err := reader.Directory(ctx, "", p1.NextCursor)
	if err != nil || len(p2.Items) != 5 || p2.NextCursor != "" {
		t.Fatalf("p2 = %d %q %v", len(p2.Items), p2.NextCursor, err)
	}
	if p1.Items[19].Username >= p2.Items[0].Username {
		t.Fatal("ordem entre páginas")
	}
	if _, err := reader.Directory(ctx, "", "lixo!"); !errors.Is(err, pagination.ErrInvalidCursor) {
		t.Fatalf("err = %v", err)
	}
}
