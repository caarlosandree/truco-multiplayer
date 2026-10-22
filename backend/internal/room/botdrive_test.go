package room_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/bot"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

// thinkCall é um pedido de cálculo guardado pelo Thinker falso; o teste dispara done quando quiser.
type thinkCall struct {
	in       bot.Input
	deadline time.Time
	done     func(bot.Decision)
}

// fakeThinker guarda os pedidos. Com auto != nil responde na hora (dentro de Think, como o fallback do
// runner pode fazer); sem auto, nunca responde sozinho.
type fakeThinker struct {
	calls []thinkCall
	auto  func(in bot.Input) bot.Decision
}

func (f *fakeThinker) Think(in bot.Input, deadline time.Time, done func(bot.Decision)) {
	f.calls = append(f.calls, thinkCall{in: in, deadline: deadline, done: done})
	if f.auto != nil {
		done(f.auto(in))
	}
}

func (f *fakeThinker) forSeat(s engine.Seat) []thinkCall {
	var out []thinkCall
	for _, c := range f.calls {
		if c.in.View.Seat == s {
			out = append(out, c)
		}
	}
	return out
}

func (f *fakeThinker) latest(t *testing.T, s engine.Seat) thinkCall {
	t.Helper()
	calls := f.forSeat(s)
	if len(calls) == 0 {
		t.Fatalf("nenhum pedido de cálculo para o assento %d", s)
	}
	return calls[len(calls)-1]
}

// firstLegal é o "cérebro" automático dos testes: a primeira ação legal.
func firstLegal(in bot.Input) bot.Decision { return bot.Decision{Action: in.Legal[0]} }

func withBots(th room.Thinker, delay time.Duration) func(*room.Options) {
	return func(o *room.Options) {
		o.Thinker = th
		o.BotDelay = func() time.Duration { return delay }
	}
}

// solo: u0 sozinho com 3 bots (b0, b1, b2 nos assentos 1, 2, 3).
var soloUsers = map[engine.Seat]string{0: "u0", 1: "b0", 2: "b1", 3: "b2"}

func newSolo(t *testing.T, th room.Thinker, delay time.Duration, mutate ...func(*room.Options)) *harness {
	t.Helper()
	h := newHarness(t, append([]func(*room.Options){withBots(th, delay)}, mutate...)...)
	h.join(0)
	startWithBots(h, "u0")
	if h.room.Status() != protocol.RoomPlaying {
		t.Fatal("a partida deveria ter começado")
	}
	return h
}

func humanUsers() map[engine.Seat]string {
	return map[engine.Seat]string{0: "u0", 1: "u1", 2: "u2", 3: "u3"}
}

func (h *harness) actAs(a engine.Action, userOf map[engine.Seat]string) {
	h.send(room.Command{UserID: userOf[a.Seat], Message: clientMsg(a)})
}

func pending(g engine.GameState, s engine.Seat) bool {
	_, ok := engine.DefaultAction(g, s)
	return ok
}

// moveOthersUntilPending joga (com carta aberta) por todos os assentos pendentes que não são target
// até target ter decisão pendente. A pausa entre mãos avança o relógio.
func (h *harness) moveOthersUntilPending(target engine.Seat, userOf map[engine.Seat]string) {
	h.t.Helper()
	for range 5000 {
		if h.room.Status() != protocol.RoomPlaying || h.room.Closed() {
			h.t.Fatal("a partida acabou antes de o assento ter decisão pendente")
		}
		g := h.room.Game()
		if pending(g, target) {
			return
		}
		if g.Phase == engine.PhaseHandEnd {
			h.advance(testTiming.HandPause)
			continue
		}
		moved := false
		for s := engine.Seat(0); s < 4; s++ {
			if a, ok := engine.DefaultAction(g, s); ok && s != target {
				a.Covered = false
				h.actAs(a, userOf)
				moved = true
				break
			}
		}
		if !moved {
			h.t.Fatalf("ninguém para jogar na fase %s", g.Phase)
		}
	}
	h.t.Fatal("o assento nunca ficou pendente")
}

func (h *harness) botActions(seat engine.Seat) int {
	n := 0
	for _, l := range h.store.last().Actions {
		if l.Source == replay.SourceBot && l.Action.Seat == seat {
			n++
		}
	}
	return n
}

func (h *harness) actionCount() int { return len(h.store.last().Actions) }

func noErrors(t *testing.T, h *harness, user ...string) {
	t.Helper()
	for _, u := range user {
		if codes := h.out.errorCodes(u); len(codes) != 0 {
			t.Fatalf("%s não deveria receber erro: %v", u, codes)
		}
	}
}

func TestBotSeatDecidesAfterDelay(t *testing.T) {
	th := &fakeThinker{}
	h := newSolo(t, th, 2*time.Second)
	h.moveOthersUntilPending(1, soloUsers)
	if n := len(th.forSeat(1)); n != 1 {
		t.Fatalf("Think para o assento 1 = %d vezes, quer 1", n)
	}
	c := th.latest(t, 1)
	before := h.actionCount()
	c.done(bot.Decision{Action: c.in.Legal[0]})
	h.drain()
	h.advance(time.Second)
	if h.actionCount() != before {
		t.Fatal("a decisão não pode ser aplicada antes do fim da pausa")
	}
	h.advance(time.Second)
	got := h.store.lastAction()
	if h.actionCount() != before+1 || got.Source != replay.SourceBot || !reflect.DeepEqual(got.Action, c.in.Legal[0]) {
		t.Fatalf("última ação = %+v (antes: %d, agora: %d)", got, before, h.actionCount())
	}
	if len(th.forSeat(1)) != 1 {
		t.Fatal("a mesma janela não pode pedir outro cálculo")
	}
}

func TestBotThinkingDoesNotBlockActor(t *testing.T) {
	th := &fakeThinker{} // nunca responde
	h := newSolo(t, th, time.Second)
	h.moveOthersUntilPending(1, soloUsers)
	h.out.reset()
	h.cmd("u0", protocol.TypeSignal, protocol.SignalPayload{ID: protocol.SignalBoa})
	if h.out.count("u0", protocol.MsgSignal) != 1 {
		t.Fatal("o sinal do humano deve ser transmitido enquanto o bot pensa")
	}
	h.cmd("u0", string(engine.ActPlayCard), protocol.PlayCardPayload{Slot: 0}) // fora da vez: erro tratado normalmente
	if len(h.out.errorCodes("u0")) != 1 {
		t.Fatal("o comando de outro assento deve ser processado (e recusado) sem esperar o bot")
	}
}

func TestStaleBotResultIsDiscarded(t *testing.T) {
	th := &fakeThinker{}
	h := newSolo(t, th, 2*time.Second)
	h.moveOthersUntilPending(1, soloUsers)
	c := th.latest(t, 1)
	a := c.in.Legal[0]
	h.actAs(a, soloUsers) // outro caminho decidiu primeiro: a janela anda
	after := h.actionCount()
	h.out.reset()
	c.done(bot.Decision{Action: a})
	h.drain()
	h.advance(5 * time.Second)
	if h.actionCount() != after || h.botActions(1) != 0 {
		t.Fatalf("resultado velho foi aplicado: %d ações (esperava %d)", h.actionCount(), after)
	}
	noErrors(t, h, "u0", "b0", "b1", "b2")
}

func TestAbsentHumanIsPlayedByBot(t *testing.T) {
	th := &fakeThinker{}
	h := newHarness(t, withBots(th, time.Second))
	h.startMatch()
	h.send(room.Disconnected{UserID: "u1"})
	if st := last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState); !st.Seats[1].BotPlaying || st.Ranked == nil || *st.Ranked {
		t.Fatalf("room_state = %+v (ranked %v)", st.Seats[1], st.Ranked)
	}
	h.moveOthersUntilPending(1, humanUsers())
	c := th.latest(t, 1)
	c.done(bot.Decision{Action: c.in.Legal[0]})
	h.drain()
	h.advance(time.Second)
	if got := h.store.lastAction(); got.Source != replay.SourceBot || got.Action.Seat != 1 {
		t.Fatalf("o ausente deveria ser jogado pelo bot: %+v", got)
	}
	h.send(room.Connected{UserID: "u1"})
	st := last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState)
	if st.Seats[1].BotPlaying || st.Ranked == nil || !*st.Ranked {
		t.Fatalf("na volta o assento é do humano e a partida conta: %+v (ranked %v)", st.Seats[1], st.Ranked)
	}
	calls := len(th.forSeat(1))
	h.moveOthersUntilPending(1, humanUsers())
	if len(th.forSeat(1)) != calls {
		t.Fatal("a decisão seguinte do humano que voltou não é do bot")
	}
}

func TestHumanActsBeforeBotDuringPause(t *testing.T) {
	th := &fakeThinker{}
	h := newHarness(t, withBots(th, 2*time.Second))
	h.startMatch()
	h.send(room.Disconnected{UserID: "u1"})
	h.moveOthersUntilPending(1, humanUsers())
	c := th.latest(t, 1)
	h.advance(time.Second)
	h.send(room.Connected{UserID: "u1"}) // volta durante a pausa de pensar
	a := c.in.Legal[0]
	h.actAs(a, humanUsers())
	if got := h.store.lastAction(); got.Source != replay.SourcePlayer || !reflect.DeepEqual(got.Action, a) {
		t.Fatalf("vale a jogada do humano: %+v", got)
	}
	n := h.actionCount()
	h.out.reset()
	c.done(bot.Decision{Action: a}) // o cálculo atrasado chega
	h.drain()
	h.advance(5 * time.Second)
	if h.botActions(1) != 0 || h.actionCount() != n {
		t.Fatalf("o resultado do bot deveria ser descartado (ações do bot: %d, ações %d -> %d)", h.botActions(1), n, h.actionCount())
	}
	noErrors(t, h, "u0", "u1", "u2", "u3")
}

func TestReturnedHumanWhoStaysIdleStillGetsBotDecision(t *testing.T) {
	th := &fakeThinker{}
	h := newHarness(t, withBots(th, 2*time.Second))
	h.startMatch()
	h.send(room.Disconnected{UserID: "u1"})
	h.moveOthersUntilPending(1, humanUsers())
	c := th.latest(t, 1)
	h.advance(time.Second)
	h.send(room.Connected{UserID: "u1"}) // volta durante a pausa e fica parado
	c.done(bot.Decision{Action: c.in.Legal[0]})
	h.drain()
	n := h.actionCount()
	h.advance(500 * time.Millisecond)
	if h.actionCount() != n {
		t.Fatal("ainda na pausa")
	}
	h.advance(500 * time.Millisecond)
	if got := h.store.lastAction(); h.actionCount() != n+1 || got.Source != replay.SourceBot || got.Action.Seat != 1 {
		t.Fatalf("a decisão pronta do bot vale até o fim da pausa: %+v", got)
	}
	if len(th.forSeat(1)) != 1 {
		t.Fatal("a volta não cria pedido novo na mesma janela")
	}
}

func TestLastCardHasShorterPause(t *testing.T) {
	h0 := newHarness(t)
	h0.startMatch()
	h0.playUntil(func(g engine.GameState) bool { return g.Phase == engine.PhasePlaying })
	g := h0.room.Game()
	turn := g.Hand.Turn
	g.Hand.Hands[turn] = g.Hand.Hands[turn][:1]
	th := &fakeThinker{}
	h := newHarness(t, withBots(th, 2*time.Second))
	id, err := h.store.Create(context.Background(), 1, 0, [4]replay.PlayerSeat{})
	if err != nil {
		t.Fatal(err)
	}
	var bots [4]bool
	bots[turn] = true
	h.room = room.NewRestored(h.opts, room.Restored{MatchID: id, State: g, Bots: bots})
	h.send(room.Wake{})
	c := th.latest(t, turn)
	c.done(bot.Decision{Action: c.in.Legal[0]})
	h.drain()
	h.advance(400 * time.Millisecond)
	if h.actionCount() != 0 {
		t.Fatal("antes de um quarto da pausa não pode agir")
	}
	h.advance(100 * time.Millisecond)
	if h.actionCount() != 1 || h.store.lastAction().Source != replay.SourceBot {
		t.Fatalf("última carta: pausa de BotDelay/4 (500 ms): %d ações", h.actionCount())
	}
}

func TestInvalidThinkerResultFallsBackToSafetyNet(t *testing.T) {
	cases := map[string]bot.Decision{
		"decisão vazia":      {},
		"ação ilegal":        {Action: engine.Action{Kind: engine.ActPlayCard, Seat: 1, Slot: 9}},
		"ação de sistema":    {Action: engine.Action{Kind: engine.ActSysForfeit, Team: 0}},
		"ação de outro lado": {Action: engine.Action{Kind: engine.ActPassBurn, Seat: 3}},
	}
	for name, d := range cases {
		t.Run(name, func(t *testing.T) {
			th := &fakeThinker{}
			h := newSolo(t, th, time.Second)
			h.moveOthersUntilPending(1, soloUsers)
			c := th.latest(t, 1)
			n := h.actionCount()
			h.out.reset()
			c.done(d)
			h.drain()
			h.advance(5 * time.Second)
			if h.actionCount() != n {
				t.Fatalf("resultado inválido não pode ser aplicado: %+v", h.store.lastAction())
			}
			noErrors(t, h, "u0", "b0", "b1", "b2")
			h.advance(testTiming.Play)
			got := h.store.last().Actions[n] // a próxima janela (outro bot) também pode vencer no avanço
			if h.actionCount() <= n || got.Source != replay.SourceTimeout || got.Action.Seat != 1 {
				t.Fatalf("no prazo a rede de segurança aplica DefaultAction: %+v", got)
			}
			h.send(room.Connected{UserID: "u0"}) // reenvia o room_state
			if st := last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState); st.Seats[1].Absent || st.Seats[1].BotPlaying {
				t.Fatalf("bot não acumula timeout: %+v", st.Seats[1])
			}
		})
	}
}

func TestConnectedInactiveBotActsAtAbsentWindowEnd(t *testing.T) {
	th := &fakeThinker{}
	h := newHarness(t, withBots(th, time.Second))
	h.startMatch()
	for i := 0; h.seatTimeouts(1) < 3; i++ {
		if i > 2000 {
			t.Fatal("assento 1 não acumulou 3 timeouts")
		}
		h.runFor(time.Second, 1)
	}
	if len(th.forSeat(1)) != 0 {
		t.Fatal("antes do 3º timeout o assento é do humano")
	}
	h.moveOthersUntilPending(1, humanUsers())
	c := th.latest(t, 1)
	if !last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState).Seats[1].Connected {
		t.Fatal("o humano segue conectado")
	}
	c.done(bot.Decision{Action: c.in.Legal[0]})
	h.drain()
	n := h.actionCount()
	h.advance(testTiming.Absent - botInactiveLead - 50*time.Millisecond) // 4,7 s
	if h.actionCount() != n {
		t.Fatal("o bot não usa a pausa curta: espera o fim da janela de ausente")
	}
	h.advance(100 * time.Millisecond)
	if got := h.store.lastAction(); h.actionCount() != n+1 || got.Source != replay.SourceBot || got.Action.Seat != 1 {
		t.Fatalf("o bot deveria agir 250 ms antes do prazo: %+v", got)
	}
}

// botInactiveLead espelha a constante da sala (250 ms).
const botInactiveLead = 250 * time.Millisecond

func TestBotSeatAccumulatesNoTimeout(t *testing.T) {
	th := &fakeThinker{} // nunca responde: só a rede de segurança do prazo
	h := newSolo(t, th, time.Second, shortAbsence)
	for i := 0; h.seatTimeouts(1) < 3; i++ {
		if i > 2000 || h.room.Status() != protocol.RoomPlaying {
			t.Fatalf("assento de bot não chegou a 3 timeouts de rede de segurança (%d)", h.seatTimeouts(1))
		}
		h.runFor(time.Second, 1, 2, 3)
	}
	st := last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState)
	if st.Seats[1].Absent || st.Seats[1].BotPlaying || st.Seats[2].Absent || st.Seats[3].Absent {
		t.Fatalf("bot nunca fica ausente: %+v", st.Seats)
	}
	if h.out.count("u0", protocol.MsgForfeitAvailable) != 0 {
		t.Fatal("bot não é alvo de W.O.")
	}
	calls := len(th.forSeat(1))
	h.moveOthersUntilPending(1, soloUsers)
	if len(th.forSeat(1)) != calls+1 {
		t.Fatal("o bot continua sendo consultado depois dos timeouts")
	}
}

func TestRankedDecidedAtFinish(t *testing.T) {
	// playOthers joga por todos os assentos fora de skip até a partida acabar; os ausentes ficam a cargo do bot.
	playOthers := func(h *harness, skip map[engine.Seat]bool, userOf map[engine.Seat]string) {
		h.t.Helper()
		for range 8000 {
			if h.room.Status() != protocol.RoomPlaying {
				return
			}
			g := h.room.Game()
			if g.Phase == engine.PhaseHandEnd {
				h.advance(testTiming.HandPause)
				continue
			}
			moved := false
			for s := engine.Seat(0); s < 4; s++ {
				if a, ok := engine.DefaultAction(g, s); ok && !skip[s] {
					a.Covered = false
					h.actAs(a, userOf)
					moved = true
					break
				}
			}
			if !moved {
				h.advance(time.Second)
			}
		}
		h.t.Fatal("a partida não terminou")
	}
	auto := func() *fakeThinker { return &fakeThinker{auto: firstLegal} }
	ended := func(h *harness) protocol.MatchEndedPayload {
		return last[protocol.MatchEndedPayload](t, h.out, "u0", protocol.MsgMatchEnded)
	}

	t.Run("humano volta antes do fim conta", func(t *testing.T) {
		h := newHarness(t, withBots(auto(), 0))
		h.startMatch()
		h.send(room.Disconnected{UserID: "u1"})
		h.advance(3 * time.Second)
		h.send(room.Connected{UserID: "u1"})
		playOthers(h, nil, humanUsers())
		if !h.store.ranked["m1"] || !ended(h).Ranked {
			t.Fatalf("ranked = %v / %v, quer true", h.store.ranked["m1"], ended(h).Ranked)
		}
	})
	t.Run("ausente até o game_over não conta", func(t *testing.T) {
		h := newHarness(t, withBots(auto(), 0))
		h.startMatch()
		h.send(room.Disconnected{UserID: "u1"})
		playOthers(h, map[engine.Seat]bool{1: true}, humanUsers())
		if v, ok := h.store.ranked["m1"]; !ok || v || ended(h).Ranked {
			t.Fatalf("ranked = %v / %v, quer false", v, ended(h).Ranked)
		}
	})
	t.Run("W.O. com bot no controle conta", func(t *testing.T) {
		h := newHarness(t, withBots(auto(), 0), shortAbsence)
		h.startMatch()
		h.send(room.Disconnected{UserID: "u1"})
		h.runFor(20*time.Second, 1)
		if r := last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState).Ranked; r == nil || *r {
			t.Fatal("com o bot no controle a partida não contaria agora")
		}
		h.cmd("u2", protocol.TypeClaimForfeit, nil)
		if v, ok := h.store.ranked["m1"]; !ok || !v || !ended(h).Ranked || ended(h).Reason != protocol.EndForfeit {
			t.Fatalf("ranked = %v / %+v, quer true", v, ended(h))
		}
	})
	t.Run("começou com bot nunca conta", func(t *testing.T) {
		h := newSolo(t, auto(), 0)
		playOthers(h, nil, soloUsers)
		if v, ok := h.store.ranked["m1"]; !ok || v || ended(h).Ranked {
			t.Fatalf("ranked = %v / %v, quer false", v, ended(h).Ranked)
		}
	})
	t.Run("começou com bot e W.O. não conta", func(t *testing.T) {
		h := newHarness(t, withBots(auto(), 0), shortAbsence)
		h.join(0)
		h.join(1)
		startWithBots(h, "u0")
		h.cmd("u1", protocol.TypeSetReady, protocol.SetReadyPayload{Ready: true})
		if h.room.Status() != protocol.RoomPlaying {
			t.Fatal("a partida deveria ter começado")
		}
		h.send(room.Disconnected{UserID: "u1"})
		h.runFor(20*time.Second, 1)
		h.cmd("u0", protocol.TypeClaimForfeit, nil)
		if v, ok := h.store.ranked["m1"]; !ok || v || ended(h).Ranked || ended(h).Reason != protocol.EndForfeit {
			t.Fatalf("ranked = %v / %+v, quer false", v, ended(h))
		}
	})
}

func TestAllAbsentCountsOnlyHumans(t *testing.T) {
	t.Run("único humano ausente abandona", func(t *testing.T) {
		h := newSolo(t, &fakeThinker{}, time.Second, shortAbsence)
		id := h.store.last().ID
		h.send(room.Disconnected{UserID: "u0"})
		h.advance(29 * time.Second)
		if h.room.Closed() {
			t.Fatal("abandono antes do orçamento")
		}
		h.advance(time.Second)
		if h.store.abandoned[id] != "all_absent" || !h.room.Closed() || h.reg.destroyed != 1 {
			t.Fatalf("abandono %q, destruída %d", h.store.abandoned[id], h.reg.destroyed)
		}
		if !h.reg.wasReleased("u0") || h.reg.wasReleased("b0") {
			t.Fatal("só o humano é liberado no lobby")
		}
	})
	t.Run("dois humanos, um ausente: nada", func(t *testing.T) {
		h := newHarness(t, withBots(&fakeThinker{}, time.Second), shortAbsence)
		h.join(0)
		h.join(1)
		startWithBots(h, "u0")
		h.cmd("u1", protocol.TypeSetReady, protocol.SetReadyPayload{Ready: true})
		h.send(room.Disconnected{UserID: "u1"})
		h.advance(2 * time.Minute)
		if h.room.Closed() || len(h.store.abandoned) != 0 {
			t.Fatal("com um humano presente a partida segue")
		}
	})
}

func TestBotSendsSignals(t *testing.T) {
	sigs := func(h *harness) []protocol.SignalBroadcastPayload {
		var out []protocol.SignalBroadcastPayload
		for _, m := range h.out.all("u0") {
			if m.Type == protocol.MsgSignal {
				out = append(out, m.Payload.(protocol.SignalBroadcastPayload))
			}
		}
		return out
	}
	truco := bot.SigTruco
	th := &fakeThinker{auto: func(in bot.Input) bot.Decision {
		d := firstLegal(in)
		if in.View.Seat == 1 {
			d.Signal = &truco
		}
		return d
	}}
	h := newSolo(t, th, 0)
	play := func(until func() bool) {
		for range 2000 {
			if until() {
				return
			}
			g := h.room.Game()
			switch {
			case g.Phase == engine.PhaseHandEnd:
				h.advance(testTiming.HandPause)
			case pending(g, 0):
				a, _ := engine.DefaultAction(g, 0)
				a.Covered = false
				h.actAs(a, soloUsers)
			default:
				t.Fatalf("ninguém joga na fase %s", g.Phase)
			}
		}
		t.Fatal("não chegou à condição")
	}
	play(func() bool { return h.botActions(1) >= 2 })
	got := sigs(h)
	if len(got) != 1 || got[0].Seat != 1 || got[0].ID != protocol.SignalTruco {
		t.Fatalf("sinais = %+v: o bot manda o sinal e respeita o intervalo de 3 s", got)
	}
	h.advance(signalIntervalForTest)
	play(func() bool { return h.botActions(1) >= 3 })
	if got := sigs(h); len(got) != 2 {
		t.Fatalf("passado o intervalo o bot pode sinalizar de novo: %+v", got)
	}
	// Sinal visto entra no Input dos bots, com a mão em que saiu.
	h.cmd("u0", protocol.TypeSignal, protocol.SignalPayload{ID: protocol.SignalVamos})
	n := len(th.calls)
	play(func() bool { return len(th.calls) > n })
	var seen []bot.SignalSeen
	for _, c := range th.calls[n:] {
		seen = c.in.Signals
	}
	hand := h.room.Game().HandNumber
	foundHuman := false
	for _, s := range seen {
		if s.Seat == 0 && s.Signal == bot.SigVamos && s.HandNumber >= 1 && s.HandNumber <= hand {
			foundHuman = true
		}
	}
	if !foundHuman {
		t.Fatalf("o sinal do humano deveria constar com HandNumber: %+v", seen)
	}
}

const signalIntervalForTest = 3 * time.Second

// Duas salas cujos estados diferem só nas mãos (e no baralho) dos outros assentos produzem o mesmo
// bot.Input para o assento 0 (anti-trapaça, spec §5.1).
func TestInputForHidesOtherHands(t *testing.T) {
	h0 := newHarness(t)
	h0.startMatch()
	h0.playUntil(func(g engine.GameState) bool { return g.Phase == engine.PhasePlaying })
	a := h0.room.Game()
	b := a.Clone()
	b.Hand.Hands[1], b.Hand.Hands[2], b.Hand.Hands[3] = b.Hand.Hands[2], b.Hand.Hands[3], b.Hand.Hands[1]
	for i, j := 0, len(b.Hand.Deck)-1; i < j; i, j = i+1, j-1 {
		b.Hand.Deck[i], b.Hand.Deck[j] = b.Hand.Deck[j], b.Hand.Deck[i]
	}
	if reflect.DeepEqual(a.Hand.Hands, b.Hand.Hands) {
		t.Fatal("os estados deveriam diferir nas mãos alheias")
	}
	mk := func(g engine.GameState) *room.Room {
		hh := newHarness(t)
		id, _ := hh.store.Create(context.Background(), 1, 0, [4]replay.PlayerSeat{})
		return room.NewRestored(hh.opts, room.Restored{MatchID: id, State: g, Players: [4]replay.PlayerSeat{{UserID: "u0"}, {UserID: "u1"}, {UserID: "u2"}, {UserID: "u3"}}})
	}
	ra, rb := mk(a), mk(b)
	for s := engine.Seat(0); s < 4; s++ {
		ia, ib := ra.InputFor(s), rb.InputFor(s)
		if s == 0 && !reflect.DeepEqual(ia, ib) {
			t.Fatalf("o Input do assento 0 depende das mãos alheias:\n%+v\n%+v", ia, ib)
		}
		if s != 0 && reflect.DeepEqual(ia.View.MyHand, ib.View.MyHand) {
			t.Fatalf("o Input do assento %d deveria refletir a própria mão", s)
		}
	}
}

func TestRestoredBotsKeepPlaying(t *testing.T) {
	th := &fakeThinker{}
	h := newHarness(t, withBots(th, time.Second))
	players := [4]engine.Player{{ID: "u0", Name: "Ana"}, {ID: "b0", Name: "Seu Zé"}, {ID: "b1", Name: "Dona Cida"}, {ID: "b2", Name: "Tio Juca"}}
	state := engine.NewGame(players, 7, 0)
	state, evs, err := engine.Apply(state, engine.Action{Kind: engine.ActSysStartHand})
	if err != nil {
		t.Fatal(err)
	}
	seats := [4]replay.PlayerSeat{{UserID: "u0", DisplayName: "Ana"}, {UserID: "b0", DisplayName: "Seu Zé", Bot: true},
		{UserID: "b1", DisplayName: "Dona Cida", Bot: true}, {UserID: "b2", DisplayName: "Tio Juca", Bot: true}}
	id, err := h.store.Create(context.Background(), 7, 0, seats)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.store.AppendAction(context.Background(), id, 0, engine.Action{Kind: engine.ActSysStartHand}, replay.SourceSystem); err != nil {
		t.Fatal(err)
	}
	rs := room.Restored{MatchID: id, Players: seats, State: state, NextPly: 1, Bots: [4]bool{false, true, true, true}}
	for s := range rs.Events {
		rs.Events[s] = engine.FilterEvents(evs, engine.Seat(s))
	}
	h.room = room.NewRestored(h.opts, rs)
	h.send(room.Wake{})

	st := last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState)
	for s := 1; s < 4; s++ {
		if !st.Seats[s].Bot || !st.Seats[s].Connected || st.Seats[s].BotPlaying {
			t.Fatalf("assento %d deveria ser bot conectado: %+v", s, st.Seats[s])
		}
	}
	if st.Seats[0].Connected || !st.Seats[0].BotPlaying || st.Ranked == nil || *st.Ranked {
		t.Fatalf("o humano restaurado volta ausente, com bot no lugar: %+v (ranked %v)", st.Seats[0], st.Ranked)
	}
	for s := engine.Seat(0); s < 4; s++ {
		if want := engine.FilterEvents(evs, s); !reflect.DeepEqual(h.room.InputFor(s).Events, want) {
			t.Fatalf("assento %d: eventos restaurados não aparecem no Input", s)
		}
	}
	for _, s := range []engine.Seat{0, 1, 2, 3} {
		if pending(h.room.Game(), s) && len(th.forSeat(s)) != 1 {
			t.Fatalf("o assento pendente %d deveria ser jogado pelo bot", s)
		}
	}
	if len(th.calls) == 0 {
		t.Fatal("nenhum cálculo pedido")
	}
	h.send(room.Connected{UserID: "u0"})
	if st := last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState); st.Seats[0].BotPlaying || st.Ranked == nil || *st.Ranked {
		t.Fatalf("na volta o assento é do humano, mas a partida com bots nunca conta: %+v (ranked %v)", st.Seats[0], st.Ranked)
	}
}

// O prazo entregue ao Thinker (o do Runner) é anterior ao da rede de segurança: um Think que devolve
// exatamente nesse prazo (fallback com o semáforo cheio) ainda é aplicado como bot, não como timeout.
func TestThinkerDeadlineLeavesRoomToApplyBeforeSafetyNet(t *testing.T) {
	th := &fakeThinker{}
	h := newSolo(t, th, 0)
	h.moveOthersUntilPending(1, soloUsers)
	c := th.latest(t, 1)
	n := h.actionCount()
	h.advance(c.deadline.Sub(h.clock.Now())) // o Runner devolve no prazo que recebeu
	if h.actionCount() != n {
		t.Fatalf("nada pode ser aplicado sem decisão: %+v", h.store.lastAction())
	}
	c.done(bot.Decision{Action: c.in.Legal[0]})
	h.drain()
	h.advance(300 * time.Millisecond)
	got := h.store.lastAction()
	if h.actionCount() != n+1 || got.Source != replay.SourceBot || got.Action.Seat != 1 {
		t.Fatalf("a decisão do Runner deveria ser aplicada antes da DefaultAction: %+v", got)
	}
	if h.seatTimeouts(1) != 0 {
		t.Fatal("o bot não acumula timeout")
	}
}

// BotDelay maior que o prazo da janela não impede o bot de agir antes da rede de segurança.
func TestBotDelayLongerThanDeadlineStillActsBeforeSafetyNet(t *testing.T) {
	th := &fakeThinker{auto: firstLegal}
	h := newSolo(t, th, time.Hour)
	h.moveOthersUntilPending(1, soloUsers)
	n := h.actionCount()
	h.advance(testTiming.Play)
	if h.botActions(1) != 1 || h.seatTimeouts(1) != 0 {
		t.Fatalf("bot deveria ter jogado antes do prazo: bot=%d timeouts=%d (ações %d -> %d)",
			h.botActions(1), h.seatTimeouts(1), n, h.actionCount())
	}
}

// Idem para o humano ausente: BotDelay > TIMER_ABSENT_SECONDS não vira timeout do humano.
func TestBotDelayLongerThanAbsentDeadlinePlaysForAbsent(t *testing.T) {
	th := &fakeThinker{auto: firstLegal}
	h := newHarness(t, withBots(th, time.Hour))
	h.startMatch()
	h.send(room.Disconnected{UserID: "u1"})
	h.moveOthersUntilPending(1, humanUsers())
	before := h.seatTimeouts(1)
	h.advance(testTiming.Absent)
	if h.botActions(1) != 1 || h.seatTimeouts(1) != before {
		t.Fatalf("o bot deveria jogar pelo ausente: bot=%d timeouts %d -> %d", h.botActions(1), before, h.seatTimeouts(1))
	}
}
