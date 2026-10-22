package room_test

import (
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/bot"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

func TestLeaveMatchAsOnlyHumanAbandons(t *testing.T) {
	h := newSolo(t, &fakeThinker{}, time.Second)
	id := h.store.last().ID
	h.out.reset()
	h.cmd("u0", protocol.TypeLeaveRoom, nil)
	if h.store.abandoned[id] != "all_left" || !h.room.Closed() || h.reg.destroyed != 1 {
		t.Fatalf("abandono %q, fechada %v, destruída %d", h.store.abandoned[id], h.room.Closed(), h.reg.destroyed)
	}
	if got := last[protocol.LeftRoomPayload](t, h.out, "u0", protocol.MsgLeftRoom); got.Reason != protocol.LeftLeft {
		t.Fatalf("left_room = %q, quer left", got.Reason)
	}
	if h.out.count("u0", protocol.MsgMatchEnded) != 0 {
		t.Fatal("quem saiu não recebe o fim da partida")
	}
	if !h.reg.wasReleased("u0") || h.reg.wasReleased("b0") {
		t.Fatal("só o humano é liberado no lobby")
	}
	noErrors(t, h, "u0")
}

func TestLeaveMatchWithOtherHumansHandsSeatToBot(t *testing.T) {
	th := &fakeThinker{}
	h := newHarness(t, withBots(th, time.Second))
	h.startMatch()
	h.out.reset()
	h.cmd("u1", protocol.TypeLeaveRoom, nil)
	if h.room.Status() != protocol.RoomPlaying || h.room.Closed() {
		t.Fatal("com outros humanos a partida segue")
	}
	if got := last[protocol.LeftRoomPayload](t, h.out, "u1", protocol.MsgLeftRoom); got.Reason != protocol.LeftLeft {
		t.Fatalf("left_room = %q, quer left", got.Reason)
	}
	if !h.reg.wasReleased("u1") {
		t.Fatal("quem saiu é liberado no lobby")
	}
	if st := last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState); !st.Seats[1].BotPlaying || !st.Seats[1].Absent {
		t.Fatalf("o assento de quem saiu fica com o bot: %+v", st.Seats[1])
	}
	h.out.reset()

	// Voltar a se conectar ou mandar comandos não devolve o assento.
	h.send(room.Connected{UserID: "u1"})
	h.cmd("u1", protocol.TypeSignal, protocol.SignalPayload{ID: protocol.SignalTruco})
	if code := lastCode(t, h.out, "u1"); code != protocol.CodeNotInRoom {
		t.Fatalf("comando de quem saiu: %q, quer not_in_room", code)
	}
	if h.out.count("u0", protocol.MsgRoomState) != 0 {
		t.Fatal("reconectar depois de sair não muda a mesa")
	}

	h.moveOthersUntilPending(1, humanUsers())
	c := th.latest(t, 1)
	c.done(bot.Decision{Action: c.in.Legal[0]})
	h.drain()
	h.advance(time.Second)
	if got := h.store.lastAction(); got.Source != replay.SourceBot || got.Action.Seat != 1 {
		t.Fatalf("o bot joga por quem saiu: %+v", got)
	}
	if n := len(h.out.all("u1")) - 1; n != 0 { // só o erro do comando acima
		t.Fatalf("quem saiu ainda recebeu %d mensagens da sala", n)
	}
}

func TestLeftSeatIsDroppedOnRematch(t *testing.T) {
	h := newHarness(t, withBots(&fakeThinker{auto: firstLegal}, 0))
	h.startMatch()
	h.cmd("u0", protocol.TypeLeaveRoom, nil) // o host sai
	if st := last[protocol.RoomStatePayload](t, h.out, "u1", protocol.MsgRoomState); st.Host != 1 {
		t.Fatalf("host = %d, quer 1 (próximo humano)", st.Host)
	}
	h.store.ranked = map[string]bool{}
	for range 8000 {
		if h.room.Status() != protocol.RoomPlaying {
			break
		}
		g := h.room.Game()
		if g.Phase == engine.PhaseHandEnd {
			h.advance(testTiming.HandPause)
			continue
		}
		moved := false
		for s := engine.Seat(1); s < 4; s++ {
			if a, ok := engine.DefaultAction(g, s); ok {
				a.Covered = false
				h.act(a)
				moved = true
				break
			}
		}
		if !moved {
			h.advance(time.Second)
		}
	}
	if h.room.Status() != protocol.RoomWaiting {
		t.Fatal("a partida deveria ter terminado")
	}
	st := last[protocol.RoomStatePayload](t, h.out, "u1", protocol.MsgRoomState)
	if st.Seats[0] != nil || st.Host != 1 {
		t.Fatalf("na revanche a cadeira de quem saiu fica vazia: %+v, host %d", st.Seats[0], st.Host)
	}
	if h.out.count("u0", protocol.MsgMatchEnded) != 0 {
		t.Fatal("quem saiu não recebe o fim da partida")
	}
}

func TestLastRemainingHumanLeavingAbandons(t *testing.T) {
	h := newHarness(t, withBots(&fakeThinker{}, time.Second))
	h.join(0)
	h.join(1)
	startWithBots(h, "u0")
	h.cmd("u1", protocol.TypeSetReady, protocol.SetReadyPayload{Ready: true})
	id := h.store.last().ID
	h.cmd("u0", protocol.TypeLeaveRoom, nil)
	if h.room.Closed() {
		t.Fatal("ainda há um humano na mesa")
	}
	h.cmd("u1", protocol.TypeLeaveRoom, nil)
	if h.store.abandoned[id] != "all_left" || !h.room.Closed() {
		t.Fatalf("abandono %q, fechada %v", h.store.abandoned[id], h.room.Closed())
	}
	if h.out.count("u0", protocol.MsgMatchEnded) != 0 || h.out.count("u1", protocol.MsgMatchEnded) != 0 {
		t.Fatal("quem saiu não recebe o fim da partida")
	}
}
