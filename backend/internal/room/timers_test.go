package room_test

import (
	"errors"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

func TestTimeoutAppliesDefaultActionWithTimeoutSource(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	cutter := h.room.Game().Cutter()
	state := last[protocol.StatePayload](t, h.out, "u0", protocol.MsgState)
	if state.Deadlines[cutter] != t0.Add(testTiming.Choice).UnixMilli() {
		t.Fatalf("prazo = %d", state.Deadlines[cutter])
	}
	h.advance(testTiming.Choice - time.Second)
	if len(h.store.last().Actions) != 1 {
		t.Fatal("disparou antes do prazo")
	}
	h.advance(time.Second)
	got := h.store.lastAction()
	if got.Source != replay.SourceTimeout || got.Action.Kind != engine.ActChooseCard || got.Action.Seat != cutter || got.Action.Keep != 0 {
		t.Fatalf("timeout gravou %+v", got)
	}
}

func TestSimultaneousVoteWindowDoesNotRearm(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	h.playUntil(func(g engine.GameState) bool { return g.Phase == engine.PhasePlaying && g.Hand.Kind == engine.HandNormal })
	turn := h.room.Game().Hand.Turn
	h.act(engine.Action{Kind: engine.ActRequestRaise, Seat: turn})
	first, second := engine.Next(turn), engine.Partner(engine.Next(turn))
	opened := h.clock.Now()
	h.advance(10 * time.Second)
	h.act(engine.Action{Kind: engine.ActVote, Seat: first, Choice: engine.VoteAccept})
	state := last[protocol.StatePayload](t, h.out, users[second], protocol.MsgState)
	if state.Deadlines[second] != opened.Add(testTiming.Vote).UnixMilli() {
		t.Fatal("o voto do parceiro não pode rearmar o prazo do outro votante")
	}
	h.advance(10 * time.Second)
	got := h.store.lastAction()
	if got.Source != replay.SourceTimeout || got.Action.Seat != second || got.Action.Choice != engine.VoteRun {
		t.Fatalf("timeout do segundo votante = %+v", got)
	}
}

func TestHandPauseStartsNextHand(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	h.playUntil(func(g engine.GameState) bool { return g.Phase == engine.PhaseHandEnd })
	hand := h.room.Game().HandNumber
	h.advance(testTiming.HandPause - time.Second)
	if h.room.Game().HandNumber != hand {
		t.Fatal("nova mão antes do fim da pausa")
	}
	h.advance(time.Second)
	got := h.store.lastAction()
	if h.room.Game().HandNumber != hand+1 || got.Action.Kind != engine.ActSysStartHand || got.Source != replay.SourceSystem {
		t.Fatalf("mão %d, última ação %+v", h.room.Game().HandNumber, got)
	}
}

func TestFullMatchFinishesAndAllowsRematch(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	first := h.store.last().ID
	h.playUntil(func(engine.GameState) bool { return false })
	if h.store.records[first].Status != replay.StatusFinished {
		t.Fatalf("status = %s", h.store.records[first].Status)
	}
	for _, u := range users {
		if e := last[protocol.MatchEndedPayload](t, h.out, u, protocol.MsgMatchEnded); e.Reason != protocol.EndGameOver || e.Winner == nil {
			t.Fatalf("%s: match_ended = %+v", u, e)
		}
	}
	if h.room.Status() != protocol.RoomWaiting {
		t.Fatal("fim da partida volta a sala para a espera")
	}
	h.readyAll()
	if h.room.Status() != protocol.RoomPlaying || h.store.last().ID == first {
		t.Fatal("revanche deve criar uma partida nova")
	}
}

func TestTimeoutPersistFailureRetries(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	h.store.failAppend = errors.New("banco fora")
	h.advance(testTiming.Choice)
	if len(h.store.last().Actions) != 1 {
		t.Fatal("nada deveria ser gravado com o banco fora")
	}
	h.store.failAppend = nil
	h.advance(time.Second)
	if len(h.store.last().Actions) != 2 {
		t.Fatal("a ação automática deve ser tentada de novo após 1s")
	}
}

func TestFinishFailureStillReturnsToWaiting(t *testing.T) {
	h := newHarness(t)
	h.store.failFinish = errors.New("banco fora")
	h.startMatch()
	id := h.store.last().ID
	h.playUntil(func(engine.GameState) bool { return false })
	if h.room.Status() != protocol.RoomWaiting || h.store.records[id].Status != replay.StatusInProgress {
		t.Fatal("falha de Finish não trava a sala; a partida fica para o Recover")
	}
}
