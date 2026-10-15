package room_test

import (
	"slices"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

// shortAbsence encurta carência e orçamento para os testes de W.O.
func shortAbsence(o *room.Options) {
	o.Timing.AbsenceGrace = 20 * time.Second
	o.Timing.AbsenceBudget = 30 * time.Second
}

// runFor simula d de jogo: assentos online jogam na hora; os da lista offline só andam por timeout.
func (h *harness) runFor(d time.Duration, offline ...engine.Seat) {
	end := h.clock.Now().Add(d)
	for h.clock.Now().Before(end) && h.room.Status() == protocol.RoomPlaying && !h.room.Closed() {
		if a, ok := nextMove(h.room.Game()); ok && !slices.Contains(offline, a.Seat) {
			h.act(a)
			continue
		}
		h.advance(time.Second)
	}
}

func (h *harness) seatTimeouts(seat engine.Seat) int {
	n := 0
	for _, l := range h.store.last().Actions {
		if l.Source == replay.SourceTimeout && l.Action.Seat == seat {
			n++
		}
	}
	return n
}

func TestThreeTimeoutsMarkSeatAbsentUntilItActs(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	for i := 0; h.seatTimeouts(1) < 3; i++ {
		if i > 2000 {
			t.Fatal("assento 1 não acumulou 3 timeouts")
		}
		h.runFor(time.Second, 1)
	}
	if !last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState).Seats[1].Absent {
		t.Fatal("3 timeouts seguidos marcam ausência")
	}
	h.playUntil(func(g engine.GameState) bool { _, ok := engine.DefaultAction(g, 1); return ok })
	a, _ := engine.DefaultAction(h.room.Game(), 1)
	a.Covered = false
	h.act(a)
	if last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState).Seats[1].Absent {
		t.Fatal("uma jogada própria encerra a ausência")
	}
}

func TestDisconnectShortensPendingDeadline(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	cutter := h.room.Game().Cutter()
	h.advance(2 * time.Second)
	h.send(room.Disconnected{UserID: users[cutter]})
	state := last[protocol.StatePayload](t, h.out, users[engine.Next(cutter)], protocol.MsgState)
	if state.Deadlines[cutter] != h.clock.Now().Add(testTiming.Absent).UnixMilli() {
		t.Fatal("o prazo de quem cai deve encurtar para TIMER_ABSENT")
	}
	h.advance(testTiming.Absent)
	if got := h.store.lastAction(); got.Source != replay.SourceTimeout || got.Action.Seat != cutter {
		t.Fatalf("última ação = %+v", got)
	}
}

func TestForfeitOfferedAfterGraceAndClaimEndsMatch(t *testing.T) {
	h := newHarness(t, shortAbsence)
	h.startMatch()
	h.send(room.Disconnected{UserID: "u1"})
	h.runFor(19*time.Second, 1)
	if h.out.count("u0", protocol.MsgForfeitAvailable) != 0 {
		t.Fatal("W.O. antes da carência")
	}
	h.runFor(time.Second, 1)
	if offer := last[protocol.ForfeitAvailablePayload](t, h.out, "u0", protocol.MsgForfeitAvailable); offer.Team == nil || *offer.Team != 1 {
		t.Fatalf("oferta = %+v", offer)
	}
	if h.out.count("u2", protocol.MsgForfeitAvailable) != 1 || h.out.count("u3", protocol.MsgForfeitAvailable) != 0 {
		t.Fatal("só a dupla adversária do ausente recebe a oferta")
	}
	h.cmd("u2", protocol.TypeClaimForfeit, nil)
	got := h.store.lastAction()
	if got.Action.Kind != engine.ActSysForfeit || got.Action.Team != 1 || got.Source != replay.SourceSystem {
		t.Fatalf("ação gravada = %+v", got)
	}
	if h.store.last().Status != replay.StatusFinished {
		t.Fatal("W.O. finaliza a partida")
	}
	if e := last[protocol.MatchEndedPayload](t, h.out, "u0", protocol.MsgMatchEnded); e.Reason != protocol.EndForfeit || *e.Winner != 0 {
		t.Fatalf("match_ended = %+v", e)
	}
	h.cmd("u0", protocol.TypeClaimForfeit, nil)
	if lastCode(t, h.out, "u0") != protocol.CodeForfeitUnavailable {
		t.Fatal("segundo pedido de W.O. deve ser recusado")
	}
}

func TestForfeitWithdrawnWhenAbsentReturns(t *testing.T) {
	h := newHarness(t, shortAbsence)
	h.startMatch()
	h.send(room.Disconnected{UserID: "u1"})
	h.runFor(20*time.Second, 1)
	h.send(room.Connected{UserID: "u1"})
	if offer := last[protocol.ForfeitAvailablePayload](t, h.out, "u0", protocol.MsgForfeitAvailable); offer.Team != nil {
		t.Fatal("a volta do ausente retira a oferta")
	}
	h.cmd("u0", protocol.TypeClaimForfeit, nil)
	if lastCode(t, h.out, "u0") != protocol.CodeForfeitUnavailable {
		t.Fatal("W.O. retirado não pode ser pedido")
	}
}

func TestForfeitBudgetAccumulatesAcrossDisconnections(t *testing.T) {
	h := newHarness(t, shortAbsence) // carência 20s, orçamento 30s
	h.startMatch()
	h.send(room.Disconnected{UserID: "u1"})
	h.runFor(15*time.Second, 1)
	h.send(room.Connected{UserID: "u1"})
	h.runFor(5 * time.Second)
	h.send(room.Disconnected{UserID: "u1"})
	h.runFor(14*time.Second, 1)
	if h.out.count("u0", protocol.MsgForfeitAvailable) != 0 {
		t.Fatal("29s acumulados ainda não estouram o orçamento de 30s")
	}
	h.runFor(time.Second, 1)
	if offer := last[protocol.ForfeitAvailablePayload](t, h.out, "u0", protocol.MsgForfeitAvailable); offer.Team == nil || *offer.Team != 1 {
		t.Fatal("o orçamento acumulado libera o W.O. antes da carência contínua")
	}
}

func TestNoForfeitWhenBothTeamsAreAbsent(t *testing.T) {
	h := newHarness(t, shortAbsence)
	h.startMatch()
	h.send(room.Disconnected{UserID: "u1"})
	h.send(room.Disconnected{UserID: "u0"})
	h.runFor(25*time.Second, 0, 1)
	if h.out.count("u2", protocol.MsgForfeitAvailable)+h.out.count("u3", protocol.MsgForfeitAvailable) != 0 {
		t.Fatal("com ausentes nas duas duplas não há W.O.")
	}
	h.cmd("u2", protocol.TypeClaimForfeit, nil)
	if lastCode(t, h.out, "u2") != protocol.CodeForfeitUnavailable {
		t.Fatal("pedido sem oferta é recusado")
	}
}

func TestReconnectResendsForfeitOffer(t *testing.T) {
	h := newHarness(t, shortAbsence)
	h.startMatch()
	h.send(room.Disconnected{UserID: "u1"})
	h.runFor(20*time.Second, 1)
	h.out.reset()
	h.send(room.Connected{UserID: "u0"}) // outra aba do mesmo usuário
	if offer := last[protocol.ForfeitAvailablePayload](t, h.out, "u0", protocol.MsgForfeitAvailable); offer.Team == nil || *offer.Team != 1 {
		t.Fatal("nova conexão deve receber a oferta vigente")
	}
}

func TestAllAbsentAbandonsAndDestroysRoom(t *testing.T) {
	h := newHarness(t, shortAbsence)
	h.startMatch()
	for _, u := range users {
		h.send(room.Disconnected{UserID: u})
	}
	h.advance(29 * time.Second)
	if h.room.Closed() {
		t.Fatal("abandono antes do orçamento")
	}
	h.advance(time.Second)
	id := h.store.last().ID
	if h.store.abandoned[id] != "all_absent" || h.reg.destroyed != 1 || !h.room.Closed() {
		t.Fatalf("abandono %q, destruída %d", h.store.abandoned[id], h.reg.destroyed)
	}
	for _, u := range users {
		if !h.reg.wasReleased(u) {
			t.Fatalf("%s deveria ser liberado", u)
		}
	}
}

func TestWaitingDisconnectRemovesSeatAfterGrace(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(1)
	h.send(room.Disconnected{UserID: "u1"})
	h.advance(59 * time.Second)
	if last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState).Seats[1] == nil {
		t.Fatal("removido antes do prazo")
	}
	h.advance(time.Second)
	if last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState).Seats[1] != nil || !h.reg.wasReleased("u1") {
		t.Fatal("desconectado em espera é removido após WAITING_DISCONNECT")
	}
}
