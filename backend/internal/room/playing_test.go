package room_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/enginetest"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

func TestAllReadyStartsMatch(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	rec := h.store.last()
	if len(rec.Actions) != 1 || rec.Actions[0].Action.Kind != engine.ActSysStartHand || rec.Actions[0].Source != replay.SourceSystem {
		t.Fatalf("log inicial = %+v", rec.Actions)
	}
	for i, u := range users {
		if rec.Players[i] != (replay.PlayerSeat{UserID: u, DisplayName: names[i]}) {
			t.Fatalf("jogador %d = %+v", i, rec.Players[i])
		}
		st := last[protocol.RoomStatePayload](t, h.out, u, protocol.MsgRoomState)
		if st.Status != protocol.RoomPlaying || st.MatchID != rec.ID {
			t.Fatalf("%s: room_state = %+v", u, st)
		}
		state := last[protocol.StatePayload](t, h.out, u, protocol.MsgState)
		if state.View.Seat != engine.Seat(i) || state.View.Phase != engine.PhaseCutChoice || len(state.PendingSeats) != 1 {
			t.Fatalf("%s: state = %+v", u, state)
		}
	}
	if sum := h.reg.lastSummary(); sum.MatchID != rec.ID || sum.Room.Status != protocol.RoomPlaying || sum.Room.Score == nil {
		t.Fatalf("resumo = %+v", sum)
	}
}

func TestPlayerActionIsPersistedBeforeBroadcast(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	before := h.out.count("u0", protocol.MsgState)
	seenAtAppend := -1
	h.store.onAppend = func() { seenAtAppend = h.out.count("u0", protocol.MsgState) }
	a, _ := nextMove(h.room.Game())
	h.act(a)
	if seenAtAppend != before || h.out.count("u0", protocol.MsgState) != before+1 {
		t.Fatalf("estado enviado antes de gravar: na gravação %d, antes %d", seenAtAppend, before)
	}
	if got := h.store.lastAction(); got.Action.Kind != a.Kind || got.Source != replay.SourcePlayer || got.Ply != 1 {
		t.Fatalf("ação gravada = %+v", got)
	}
}

func TestRuleErrorKeepsStateAndLogNothing(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	g := h.room.Game()
	intruder := engine.Next(g.Cutter())
	illegal := engine.Action{Kind: engine.ActChooseCard, Seat: intruder, Keep: 0}
	_, _, want := engine.Apply(g, illegal)
	var ruleErr *engine.RuleError
	if !errors.As(want, &ruleErr) {
		t.Fatalf("a ação deveria ser ilegal: %v", want)
	}
	h.act(illegal)
	p := last[protocol.ErrorPayload](t, h.out, users[intruder], protocol.MsgError)
	if p.Code != string(ruleErr.Code) || p.Seq != 3 {
		t.Fatalf("erro = %+v", p)
	}
	if len(h.store.last().Actions) != 1 || !reflect.DeepEqual(h.room.Game(), g) {
		t.Fatal("jogada ilegal não pode gravar nem mudar o estado")
	}
}

func TestAppendFailureDiscardsState(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	g := h.room.Game()
	a, _ := nextMove(g)
	h.store.failAppend = errors.New("banco fora")
	h.act(a)
	if lastCode(t, h.out, users[a.Seat]) != protocol.CodeInternal || !reflect.DeepEqual(h.room.Game(), g) {
		t.Fatal("falha de gravação deve descartar o estado e responder internal")
	}
	h.store.failAppend = nil
	h.act(a)
	if len(h.store.last().Actions) != 2 {
		t.Fatal("a mesma jogada deve funcionar depois que o banco volta")
	}
}

func TestClosedMatchEndsAsAborted(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	h.store.last().Status = replay.StatusAbandoned
	a, _ := nextMove(h.room.Game())
	h.act(a)
	for _, u := range users {
		if e := last[protocol.MatchEndedPayload](t, h.out, u, protocol.MsgMatchEnded); e.Reason != protocol.EndAborted || e.Winner != nil {
			t.Fatalf("%s: match_ended = %+v", u, e)
		}
	}
	st := last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState)
	if h.room.Status() != protocol.RoomWaiting || st.MatchID != "" {
		t.Fatalf("sala deve voltar a esperar: %+v", st)
	}
	for _, seat := range st.Seats {
		if seat.Ready {
			t.Fatal("pronto deve ser desmarcado")
		}
	}
}

func TestAbortMessageEndsMatch(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	h.send(room.Abort{Reason: "admin"})
	if e := last[protocol.MatchEndedPayload](t, h.out, "u2", protocol.MsgMatchEnded); e.Reason != protocol.EndAborted {
		t.Fatalf("match_ended = %+v", e)
	}
	if h.room.Status() != protocol.RoomWaiting {
		t.Fatal("abort volta a sala para a espera")
	}
}

func TestCreateFailureReturnsToWaiting(t *testing.T) {
	h := newHarness(t)
	h.store.failCreate = errors.New("banco fora")
	h.seatAll()
	h.readyAll()
	if h.room.Status() != protocol.RoomWaiting || lastCode(t, h.out, "u3") != protocol.CodeInternal {
		t.Fatal("falha ao criar a partida volta a esperar com erro interno")
	}
	for _, seat := range last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState).Seats {
		if seat.Ready {
			t.Fatal("pronto deve ser desmarcado")
		}
	}
}

func TestStartHandFailureAbandonsCreatedMatch(t *testing.T) {
	h := newHarness(t)
	h.store.failAppend = errors.New("banco fora")
	h.seatAll()
	h.readyAll()
	id := h.store.last().ID
	if h.room.Status() != protocol.RoomWaiting || h.store.abandoned[id] != "start_failed" {
		t.Fatalf("status %s, abandono %q", h.room.Status(), h.store.abandoned[id])
	}
}

func TestRestoredRoomResumesWithSeatsOffline(t *testing.T) {
	full, _ := enginetest.RandomRecord(9)
	rec := full
	var state engine.GameState
	plies := 15
	for ; ; plies++ { // corta o log num ponto em que alguém precisa decidir
		rec.Actions = full.Actions[:plies]
		var err error
		if state, err = replay.Rebuild(rec); err != nil {
			t.Fatal(err)
		}
		if _, ok := nextMove(state); ok {
			break
		}
	}
	h := newHarness(t)
	h.store.records[rec.ID] = &rec
	h.store.order = append(h.store.order, rec.ID)
	h.room = room.NewRestored(h.opts, room.Restored{MatchID: rec.ID, Players: rec.Players, State: state, NextPly: plies})
	h.send(room.Wake{})
	if h.room.Status() != protocol.RoomPlaying {
		t.Fatal("sala restaurada começa em jogo")
	}
	h.send(room.Connected{UserID: "u2"})
	st := last[protocol.RoomStatePayload](t, h.out, "u2", protocol.MsgRoomState)
	if st.MatchID != rec.ID || !st.Seats[2].Connected || st.Seats[0].Connected || st.Host != 0 {
		t.Fatalf("room_state = %+v", st)
	}
	view := last[protocol.StatePayload](t, h.out, "u2", protocol.MsgState).View
	if !reflect.DeepEqual(view, engine.ViewFor(state, 2)) {
		t.Fatal("reconexão deve receber a view do estado restaurado")
	}
	a, _ := nextMove(h.room.Game())
	h.send(room.Connected{UserID: users[a.Seat]})
	h.act(a)
	if got := h.store.lastAction(); got.Ply != plies {
		t.Fatalf("próxima ação gravada no ply %d, quer %d", got.Ply, plies)
	}
}
