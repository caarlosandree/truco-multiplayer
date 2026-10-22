package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/admin"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/bot"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/httpapi"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/lobby"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/transport/ws"
)

// Sem pausa entre mãos e sem timeouts (o relógio manual só anda quando o teste manda).
var e2eTiming = room.Timing{Play: 30 * time.Second, Vote: 20 * time.Second, Choice: 15 * time.Second,
	Burn: 15 * time.Second, Absent: 5 * time.Second, HandPause: 0, AbsenceGrace: 20 * time.Second,
	AbsenceBudget: 50 * time.Second, WaitingDisconnect: time.Minute}

// swapHandler permite trocar o lobby atrás do /ws (teste de restart).
type swapHandler struct{ current atomic.Pointer[ws.Handler] }

func (s *swapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.current.Load().ServeHTTP(w, r)
}

type game struct {
	h      *harness
	auth   *auth.Service
	ws     *swapHandler
	lobby  *lobby.Lobby
	limits ws.Limits
	solo   bool // lobby com roster de bots e runner (start_with_bots)
}

// fastBots libera o limite de mensagens: bots jogam a partida inteira em menos de um segundo,
// o que um humano (20 msg/s, rajada de 40) nunca faria.
func fastBots(l *ws.Limits) { l.RatePerSec, l.Burst = 10000, 10000 }

func newGame(t *testing.T, tweak ...func(*ws.Limits)) *game { return newGameMode(t, false, tweak...) }

func newGameMode(t *testing.T, solo bool, tweak ...func(*ws.Limits)) *game {
	g := &game{ws: &swapHandler{}, limits: ws.DefaultLimits, solo: solo}
	for _, f := range tweak {
		f(&g.limits)
	}
	g.h = newHarness(t, func(d *httpapi.Deps) {
		g.auth = d.Auth
		d.WS = g.ws
		d.Admin = admin.NewService(admin.Options{Pool: d.Pool, Auth: d.Auth, Aborter: g, Now: d.Now, Logger: d.Logger})
	})
	g.startLobby()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = g.lobby.Shutdown(ctx)
	})
	return g
}

// Abort delega ao lobby corrente.
func (g *game) Abort(ctx context.Context, id string) error { return g.lobby.Abort(ctx, id) }

func (g *game) startLobby() {
	discard := slog.New(slog.NewTextHandler(io.Discard, nil))
	timing := e2eTiming
	if g.solo {
		// o único humano fica ausente (3 timeouts) até o fim da partida: sem isto o orçamento de ausência
		// (todos os humanos ausentes) a abandonaria antes do fim. O relógio é manual, então não custa tempo real.
		timing.AbsenceGrace, timing.AbsenceBudget = 6*time.Hour, 6*time.Hour
	}
	opts := lobby.Options{Timing: timing, MaxRooms: 10,
		Store: lobby.NewMatchStore(g.h.store), Clock: g.h.clock, Rand: rand.NewChaCha8([32]byte{7}), Log: discard}
	if g.solo {
		bots, err := g.h.store.ListBots(context.Background())
		if err != nil || len(bots) < 3 {
			g.h.t.Fatalf("roster de bots: %d (%v)", len(bots), err)
		}
		for _, b := range bots {
			opts.Bots = append(opts.Bots, room.BotIdentity{UserID: b.ID, DisplayName: b.DisplayName})
		}
		// BOT_DELAY_*=0 (BotDelay nil) e BOT_THINK_BUDGET_MS=50. O relógio do teste é manual (e fica no
		// passado em relação ao relógio real), então o runner cai no bot simples: o que se valida aqui é
		// a orquestração (assumir ausente, persistir source='bot', ranked=false), não a força do bot.
		opts.Thinker = bot.NewRunner(bot.RunnerOptions{Strong: bot.DefaultStrong(), Fallback: bot.Simple{},
			Budget: 50 * time.Millisecond, MaxConcurrent: 2})
	}
	g.lobby = lobby.New(context.Background(), opts)
	g.ws.current.Store(&ws.Handler{Auth: g.auth, Lobby: g.lobby, Origin: origin, Limits: g.limits, Log: discard})
}

type inbound struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type player struct {
	t    *testing.T
	conn *websocket.Conn
	in   chan inbound
	err  error // motivo do fim da leitura; válido depois que in fecha
}

func (g *game) connect(c *client) *player {
	g.h.t.Helper()
	u, _ := url.Parse(g.h.srv.URL)
	header := http.Header{"Origin": {origin}}
	for _, ck := range c.http.Jar.Cookies(u) {
		header.Add("Cookie", ck.Name+"="+ck.Value)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(g.h.srv.URL, "http")+"/ws", &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		g.h.t.Fatalf("dial: %v", err)
	}
	p := &player{t: g.h.t, conn: conn, in: make(chan inbound, 1024)}
	go func() {
		defer close(p.in)
		for {
			_, data, err := conn.Read(context.Background())
			if err != nil {
				p.err = err
				return
			}
			var m inbound
			if json.Unmarshal(data, &m) == nil {
				p.in <- m
			}
		}
	}()
	g.h.t.Cleanup(func() { _ = conn.CloseNow() })
	return p
}

func (p *player) send(typ string, payload any) {
	msg := map[string]any{"type": typ, "seq": 1}
	if payload != nil {
		msg["payload"] = payload
	}
	data, _ := json.Marshal(msg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.conn.Write(ctx, websocket.MessageText, data); err != nil {
		p.t.Fatalf("enviar %s: %v", typ, err)
	}
}

// expect descarta mensagens até chegar uma do tipo e decodifica o payload em into (se não nil).
func (p *player) expect(typ string, into any) {
	p.t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case m, ok := <-p.in:
			if !ok {
				p.t.Fatalf("conexão fechou esperando %s: %v", typ, p.err)
			}
			if m.Type != typ {
				continue
			}
			if into != nil {
				if err := json.Unmarshal(m.Payload, into); err != nil {
					p.t.Fatal(err)
				}
			}
			return
		case <-timeout:
			p.t.Fatalf("não chegou %s", typ)
		}
	}
}

func (p *player) closedWith() websocket.StatusCode {
	p.t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-p.in:
			if !ok {
				return websocket.CloseStatus(p.err)
			}
		case <-timeout:
			p.t.Fatal("a conexão não fechou")
			return 0
		}
	}
}

// startMatch cadastra quatro usuários, forma uma sala pública e espera a partida começar.
func (g *game) startMatch() ([]*player, []*client, protocol.RoomStatePayload) {
	g.h.t.Helper()
	var players []*player
	var clients []*client
	for _, name := range []string{"ana", "bia", "caio", "duda"} {
		c := g.h.signup(name)
		p := g.connect(c)
		p.expect("hello", nil)
		clients, players = append(clients, c), append(players, p)
	}
	players[0].send("create_room", map[string]any{"visibility": "public"})
	var created protocol.RoomStatePayload
	players[0].expect("room_state", &created)
	for _, p := range players[1:] {
		p.send("join_room", map[string]any{"roomId": created.ID})
		p.expect("room_state", nil)
	}
	for _, p := range players {
		p.send("set_ready", map[string]any{"ready": true})
	}
	var playing protocol.RoomStatePayload
	for playing.Status != protocol.RoomPlaying {
		players[0].expect("room_state", &playing)
	}
	return players, clients, playing
}

func legalMove(v engine.PlayerView) (string, any) {
	switch v.Phase {
	case engine.PhaseCutChoice, engine.PhaseDealerChoice:
		return "choose_card", map[string]int{"keep": 0}
	case engine.PhaseBurnFours:
		return "pass_burn", nil
	case engine.PhaseElevenDecision:
		return "vote", map[string]string{"choice": "play"}
	case engine.PhaseAwaitingRaiseResponse:
		return "vote", map[string]string{"choice": "accept"}
	}
	return "play_card", map[string]any{"slot": v.MyHand[0].Slot, "covered": false}
}

// playToEnd responde a cada state em que o jogador está pendente até todos receberem match_ended.
func (g *game) playToEnd(players []*player) protocol.MatchEndedPayload {
	g.h.t.Helper()
	type tagged struct {
		i int
		m inbound
	}
	merged := make(chan tagged, 4096)
	stop := make(chan struct{})
	defer close(stop)
	for i, p := range players {
		go func() {
			for m := range p.in {
				select {
				case merged <- tagged{i, m}:
				case <-stop:
					return
				}
			}
		}()
	}
	var result protocol.MatchEndedPayload
	timeout := time.After(60 * time.Second)
	for ended := 0; ended < len(players); {
		select {
		case x := <-merged:
			switch x.m.Type {
			case "state":
				var st protocol.StatePayload
				if err := json.Unmarshal(x.m.Payload, &st); err != nil {
					g.h.t.Fatal(err)
				}
				if slices.Contains(st.PendingSeats, int(st.View.Seat)) {
					typ, payload := legalMove(st.View)
					players[x.i].send(typ, payload)
				}
			case "match_ended":
				ended++
				_ = json.Unmarshal(x.m.Payload, &result)
			}
		case <-timeout:
			g.h.t.Fatal("a partida travou")
		}
	}
	return result
}

func (g *game) matchRow(id string) (status string, winner *int16) {
	g.h.t.Helper()
	if err := g.h.pool.QueryRow(context.Background(), "SELECT status, winner_team FROM matches WHERE id = $1", id).Scan(&status, &winner); err != nil {
		g.h.t.Fatal(err)
	}
	return status, winner
}

func TestWebSocketFullMatchIsPersisted(t *testing.T) {
	g := newGame(t, fastBots)
	players, _, started := g.startMatch()
	result := g.playToEnd(players)
	if result.Reason != protocol.EndGameOver || result.Winner == nil {
		t.Fatalf("match_ended = %+v", result)
	}
	status, winner := g.matchRow(started.MatchID)
	if status != "finished" || winner == nil || int(*winner) != int(*result.Winner) {
		t.Fatalf("partida no banco: %s %v", status, winner)
	}
}

func TestWebSocketReconnectReceivesSnapshot(t *testing.T) {
	g := newGame(t)
	players, clients, started := g.startMatch()
	_ = players[1].conn.CloseNow()
	again := g.connect(clients[1])
	again.expect("hello", nil)
	var st protocol.RoomStatePayload
	again.expect("room_state", &st)
	if st.Status != protocol.RoomPlaying || st.YourSeat != 1 || st.MatchID != started.MatchID {
		t.Fatalf("room_state = %+v", st)
	}
	var state protocol.StatePayload
	again.expect("state", &state)
	if state.View.Seat != 1 {
		t.Fatalf("state = %+v", state.View)
	}
}

func TestWebSocketForfeit(t *testing.T) {
	g := newGame(t)
	players, _, started := g.startMatch()
	_ = players[1].conn.CloseNow()
	for {
		var st protocol.RoomStatePayload
		players[0].expect("room_state", &st)
		if !st.Seats[1].Connected {
			break
		}
	}
	for range 25 {
		g.h.clock.Advance(time.Second)
	}
	var offer protocol.ForfeitAvailablePayload
	players[0].expect("forfeit_available", &offer)
	if offer.Team == nil || *offer.Team != 1 {
		t.Fatalf("oferta = %+v", offer)
	}
	players[0].send("claim_forfeit", nil)
	var ended protocol.MatchEndedPayload
	players[0].expect("match_ended", &ended)
	if ended.Reason != protocol.EndForfeit || *ended.Winner != 0 {
		t.Fatalf("match_ended = %+v", ended)
	}
	if status, winner := g.matchRow(started.MatchID); status != "finished" || *winner != 0 {
		t.Fatalf("partida no banco: %s %v", status, winner)
	}
}

func TestWebSocketAdminAbort(t *testing.T) {
	g := newGame(t)
	players, clients, started := g.startMatch()
	g.h.makeAdmin("ana")
	if r := clients[0].post("/api/admin/matches/"+started.MatchID+"/abort", map[string]string{"reason": "teste"}); r.status != http.StatusNoContent {
		t.Fatalf("abort = %d %s", r.status, r.body)
	}
	for _, p := range players {
		var ended protocol.MatchEndedPayload
		p.expect("match_ended", &ended)
		if ended.Reason != protocol.EndAborted {
			t.Fatalf("match_ended = %+v", ended)
		}
	}
}

func TestWebSocketRestartResumesMatch(t *testing.T) {
	g := newGame(t, fastBots)
	players, clients, started := g.startMatch()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := g.lobby.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	for _, p := range players {
		if code := p.closedWith(); code != websocket.StatusGoingAway {
			t.Fatalf("fechamento no desligamento = %d", code)
		}
	}
	recovered, err := g.h.store.Recover(ctx, nil)
	if err != nil || len(recovered) != 1 || recovered[0].Record.ID != started.MatchID {
		t.Fatalf("recuperadas = %+v (%v)", recovered, err)
	}
	g.startLobby()
	if err := g.lobby.Restore(recovered); err != nil {
		t.Fatal(err)
	}
	var again []*player
	for i, c := range clients {
		p := g.connect(c)
		p.expect("hello", nil)
		var st protocol.RoomStatePayload
		p.expect("room_state", &st)
		if st.Status != protocol.RoomPlaying || int(st.YourSeat) != i || st.MatchID != started.MatchID {
			t.Fatalf("room_state = %+v", st)
		}
		again = append(again, p)
	}
	if result := g.playToEnd(again); result.Reason != protocol.EndGameOver {
		t.Fatalf("match_ended = %+v", result)
	}
	if status, _ := g.matchRow(started.MatchID); status != "finished" {
		t.Fatalf("status = %s", status)
	}
}

func TestWebSocketLogoutAllClosesConnection(t *testing.T) {
	g := newGame(t, func(l *ws.Limits) { l.Revalidate = 50 * time.Millisecond })
	c := g.h.signup("ana")
	p := g.connect(c)
	p.expect("hello", nil)
	if r := c.post("/api/auth/logout-all", nil); r.status != http.StatusNoContent {
		t.Fatalf("logout-all = %d %s", r.status, r.body)
	}
	if code := p.closedWith(); code != 4003 {
		t.Fatalf("fechamento = %d", code)
	}
}

// Partida solo: um humano cria a sala, começa com bots, joga uma carta e some. Após 3 timeouts o bot
// assume o assento dele e a partida termina fora do ranking, sem estatísticas e com ações source='bot'.
func TestSoloMatchWithBotsFinishesUnranked(t *testing.T) {
	g := newGameMode(t, true, fastBots)
	c := g.h.signup("ana")
	p := g.connect(c)
	p.expect("hello", nil)
	p.send("create_room", map[string]any{"visibility": "private"})
	p.expect("room_state", nil)
	p.send("start_with_bots", nil)
	var playing protocol.RoomStatePayload
	for playing.Status != protocol.RoomPlaying {
		p.expect("room_state", &playing)
	}
	for _, s := range playing.Seats[1:] {
		if s == nil || !s.Bot {
			t.Fatalf("assentos = %+v", playing.Seats)
		}
	}

	// uma única jogada humana; depois o teste só avança o relógio e lê o que chega
	played := false
	var result protocol.MatchEndedPayload
	deadline := time.Now().Add(90 * time.Second)
	for done := false; !done; {
		if time.Now().After(deadline) {
			t.Fatal("a partida solo não terminou")
		}
		g.h.clock.Advance(200 * time.Millisecond)
		time.Sleep(2 * time.Millisecond)
	drain:
		for {
			select {
			case m, ok := <-p.in:
				if !ok {
					t.Fatalf("conexão fechou: %v", p.err)
				}
				switch m.Type {
				case "state":
					var st protocol.StatePayload
					if err := json.Unmarshal(m.Payload, &st); err != nil {
						t.Fatal(err)
					}
					if !played && slices.Contains(st.PendingSeats, int(st.View.Seat)) {
						typ, payload := legalMove(st.View)
						p.send(typ, payload)
						played = true
					}
				case "match_ended":
					_ = json.Unmarshal(m.Payload, &result)
					done = true
				}
			default:
				break drain
			}
		}
	}
	if !played {
		t.Fatal("o humano nunca ficou pendente")
	}
	if result.Reason != protocol.EndGameOver || result.Ranked {
		t.Fatalf("match_ended = %+v", result)
	}

	ctx := context.Background()
	var status string
	var ranked bool
	if err := g.h.pool.QueryRow(ctx, "SELECT status, ranked FROM matches WHERE id = $1", playing.MatchID).Scan(&status, &ranked); err != nil {
		t.Fatal(err)
	}
	if status != "finished" || ranked {
		t.Fatalf("partida no banco: status=%s ranked=%v", status, ranked)
	}
	var botActions, stats int
	if err := g.h.pool.QueryRow(ctx, "SELECT count(*) FROM match_actions WHERE match_id = $1 AND source = 'bot'", playing.MatchID).Scan(&botActions); err != nil {
		t.Fatal(err)
	}
	if botActions == 0 {
		t.Fatal("nenhuma ação com source='bot'")
	}
	if err := g.h.pool.QueryRow(ctx, "SELECT count(*) FROM match_player_stats WHERE match_id = $1", playing.MatchID).Scan(&stats); err != nil {
		t.Fatal(err)
	}
	if stats != 0 {
		t.Fatalf("partida fora do ranking gerou %d linhas em match_player_stats", stats)
	}
	if err := g.h.pool.QueryRow(ctx, "SELECT count(*) FROM match_player_stats").Scan(&stats); err != nil || stats != 0 {
		t.Fatalf("match_player_stats não está vazia: %d (%v)", stats, err)
	}
	r := c.get("/api/ranking")
	if r.status != http.StatusOK || strings.Contains(string(r.body), "ana") {
		t.Fatalf("ranking = %d %s", r.status, r.body)
	}
}
