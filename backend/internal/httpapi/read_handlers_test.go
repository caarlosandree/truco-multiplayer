package httpapi_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/enginetest"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

type table struct {
	clients [4]*client
	players [4]replay.PlayerSeat
}

func (h *harness) table() table {
	h.t.Helper()
	var tb table
	for i, name := range []string{"ana", "bia", "caio", "duda"} {
		tb.clients[i] = h.signup(name)
		tb.players[i] = replay.PlayerSeat{UserID: tb.clients[i].userID(), DisplayName: name}
	}
	return tb
}

// playMatch grava uma partida; actions < 0 grava todas e finaliza.
func (h *harness) playMatch(tb table, seed uint64, actions int) (string, replay.Record) {
	h.t.Helper()
	ctx := context.Background()
	rec, _ := enginetest.RandomRecord(seed)
	id, err := h.store.Create(ctx, rec.Seed, rec.FirstDealer, tb.players)
	if err != nil {
		h.t.Fatal(err)
	}
	if actions >= 0 {
		rec.Actions = rec.Actions[:actions]
	}
	for _, a := range rec.Actions {
		if err := h.store.AppendAction(ctx, id, a.Ply, a.Action, a.Source); err != nil {
			h.t.Fatal(err)
		}
	}
	if actions < 0 {
		if err := h.store.Finish(ctx, id); err != nil {
			h.t.Fatal(err)
		}
	}
	return id, rec
}

func TestReadRoutesRequireSession(t *testing.T) {
	h := newHarness(t)
	anon := h.client()
	for _, path := range []string{"/api/ranking", "/api/users/ana", "/api/matches", "/api/matches/x", "/api/matches/x/replay"} {
		expect(t, anon.get(path), http.StatusUnauthorized, "unauthenticated")
	}
}

func TestRankingProfileAndHistory(t *testing.T) {
	h := newHarness(t)
	tb := h.table()
	h.playMatch(tb, 1, -1)
	h.playMatch(tb, 2, -1)
	ana := tb.clients[0]

	var ranking struct {
		Items []struct {
			Position      int    `json:"position"`
			Username      string `json:"username"`
			MatchesPlayed int    `json:"matchesPlayed"`
		} `json:"items"`
	}
	r := ana.get("/api/ranking?limit=2")
	r.decode(t, &ranking)
	if r.status != 200 || len(ranking.Items) != 2 || ranking.Items[0].Position != 1 || ranking.Items[0].MatchesPlayed != 2 {
		t.Fatalf("ranking = %d %s", r.status, r.body)
	}

	var profile struct {
		Username string `json:"username"`
		Stats    struct {
			MatchesPlayed int `json:"matchesPlayed"`
		} `json:"stats"`
	}
	r = ana.get("/api/users/bia")
	r.decode(t, &profile)
	if r.status != 200 || profile.Username != "bia" || profile.Stats.MatchesPlayed != 2 {
		t.Fatalf("perfil = %d %s", r.status, r.body)
	}
	expect(t, ana.get("/api/users/ninguem"), http.StatusNotFound, "not_found")

	var page struct {
		Items      []struct{ ID string } `json:"items"`
		NextCursor string                `json:"nextCursor"`
	}
	r = ana.get("/api/matches")
	r.decode(t, &page)
	if r.status != 200 || len(page.Items) != 2 {
		t.Fatalf("histórico próprio = %d %s", r.status, r.body)
	}
	if r := ana.get("/api/matches?user=caio"); r.status != 200 {
		t.Fatalf("histórico de outro = %d", r.status)
	}
	expect(t, ana.get("/api/matches?cursor=lixo!"), http.StatusBadRequest, "validation_failed")
	expect(t, ana.get("/api/matches?user=ninguem"), http.StatusNotFound, "not_found")
}

func TestMatchSummary(t *testing.T) {
	h := newHarness(t)
	tb := h.table()
	id, _ := h.playMatch(tb, 3, -1)
	var summary struct {
		Status  string `json:"status"`
		Players []struct {
			Seat     int    `json:"seat"`
			Username string `json:"username"`
		} `json:"players"`
		Score *[2]int `json:"score"`
	}
	r := tb.clients[0].get("/api/matches/" + id)
	r.decode(t, &summary)
	if r.status != 200 || summary.Status != "finished" || len(summary.Players) != 4 || summary.Score == nil {
		t.Fatalf("resumo = %d %s", r.status, r.body)
	}
	expect(t, tb.clients[0].get("/api/matches/00000000-0000-7000-8000-000000000000"), http.StatusNotFound, "not_found")
	expect(t, tb.clients[0].get("/api/matches/nao-e-uuid"), http.StatusNotFound, "not_found")
}

func TestReplay(t *testing.T) {
	h := newHarness(t)
	tb := h.table()
	id, rec := h.playMatch(tb, 4, -1)
	var replayBody struct {
		ID      string `json:"id"`
		Players []struct {
			Seat        int    `json:"seat"`
			DisplayName string `json:"displayName"`
		} `json:"players"`
		Steps []struct {
			Ply    int                  `json:"ply"`
			Views  [4]engine.PlayerView `json:"views"`
			Events []map[string]any     `json:"events"`
		} `json:"steps"`
	}
	r := tb.clients[0].get("/api/matches/" + id + "/replay")
	r.decode(t, &replayBody)
	if r.status != 200 || len(replayBody.Steps) != len(rec.Actions) || len(replayBody.Players) != 4 {
		t.Fatalf("replay = %d, %d passos (quer %d)", r.status, len(replayBody.Steps), len(rec.Actions))
	}
	last := replayBody.Steps[len(replayBody.Steps)-1]
	if last.Views[0].Phase != engine.PhaseGameOver {
		t.Fatalf("último passo na fase %s", last.Views[0].Phase)
	}

	gz := tb.clients[0].get("/api/matches/"+id+"/replay", "Accept-Encoding", "gzip")
	if gz.header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("sem gzip: %v", gz.header)
	}
	zr, err := gzip.NewReader(bytes.NewReader(gz.body))
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := io.ReadAll(zr)
	if !json.Valid(plain) {
		t.Fatal("corpo gzip não é JSON válido")
	}
}

func TestReplayUnavailableCases(t *testing.T) {
	h := newHarness(t)
	tb := h.table()
	ctx := context.Background()
	inProgress, _ := h.playMatch(tb, 5, 10)
	expect(t, tb.clients[0].get("/api/matches/"+inProgress+"/replay"), http.StatusConflict, "match_in_progress")

	corrupt, _ := h.playMatch(tb, 6, 1)
	if err := h.store.AppendAction(ctx, corrupt, 1, engine.Action{Kind: engine.ActPlayCard, Seat: 0}, replay.SourcePlayer); err != nil {
		t.Fatal(err)
	}
	if err := h.store.Abandon(ctx, corrupt, "corrupt_log"); err != nil {
		t.Fatal(err)
	}
	expect(t, tb.clients[0].get("/api/matches/"+corrupt+"/replay"), http.StatusConflict, "conflict")
}

func TestReplayEventsNeverNull(t *testing.T) {
	h := newHarness(t)
	tb := h.table()
	id, _ := h.playMatch(tb, 7, -1)
	r := tb.clients[0].get("/api/matches/" + id + "/replay")
	var body struct {
		Players []json.RawMessage `json:"players"`
		Steps   []struct {
			Events json.RawMessage `json:"events"`
		} `json:"steps"`
	}
	r.decode(t, &body)
	for i, s := range body.Steps {
		if string(s.Events) == "null" {
			t.Fatalf("passo %d com events null", i)
		}
	}
}

func TestDisabledUserHiddenFromRankingButKeepsProfile(t *testing.T) {
	h := newHarness(t)
	tb := h.table()
	h.playMatch(tb, 8, -1)
	if _, err := h.pool.Exec(context.Background(), "UPDATE users SET disabled_at = now() WHERE username = 'duda'"); err != nil {
		t.Fatal(err)
	}
	var ranking struct {
		Items []struct {
			Username string `json:"username"`
		} `json:"items"`
	}
	tb.clients[0].get("/api/ranking").decode(t, &ranking)
	if len(ranking.Items) != 3 {
		t.Fatalf("ranking deveria omitir o desativado: %+v", ranking.Items)
	}
	expect(t, tb.clients[0].get("/api/users/duda"), http.StatusNotFound, "not_found")
	expect(t, tb.clients[0].get("/api/matches?user=duda"), http.StatusNotFound, "not_found")
	h.makeAdmin("ana")
	if r := tb.clients[0].get("/api/users/duda"); r.status != http.StatusOK {
		t.Fatalf("perfil de desativado para admin = %d", r.status)
	}
	if r := tb.clients[0].get("/api/matches?user=duda"); r.status != http.StatusOK {
		t.Fatalf("histórico de desativado para admin = %d", r.status)
	}
}

func TestDirectoryRoute(t *testing.T) {
	h := newHarness(t)
	expect(t, h.client().get("/api/users"), http.StatusUnauthorized, "unauthenticated")
	tb := h.table()
	var page struct {
		Items []map[string]any `json:"items"`
	}
	r := tb.clients[0].get("/api/users?q=bi")
	if r.status != http.StatusOK {
		t.Fatalf("status = %d", r.status)
	}
	r.decode(t, &page)
	if len(page.Items) != 1 || page.Items[0]["username"] != "bia" || len(page.Items[0]) != 2 {
		t.Fatalf("items = %+v", page.Items)
	}
	expect(t, tb.clients[0].get("/api/users?q=a%00b"), http.StatusBadRequest, "validation_failed")
	expect(t, tb.clients[0].get("/api/users?q=%ff"), http.StatusBadRequest, "validation_failed")
	expect(t, tb.clients[0].get("/api/users?q="+strings.Repeat("a", 65)), http.StatusBadRequest, "validation_failed")
	expect(t, tb.clients[0].get("/api/users?cursor=AA"), http.StatusBadRequest, "validation_failed")
	expect(t, tb.clients[0].get("/api/users?cursor=lixo!"), http.StatusBadRequest, "validation_failed")
}
