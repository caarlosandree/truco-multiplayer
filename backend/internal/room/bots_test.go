package room_test

import (
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

func startWithBots(h *harness, user string) { h.cmd(user, protocol.TypeStartWithBots, nil) }

func TestStartWithBotsFillsAndStarts(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	startWithBots(h, "u0")
	if h.room.Status() != protocol.RoomPlaying {
		t.Fatal("a partida deveria ter começado")
	}
	st := last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState)
	names := map[string]bool{}
	for s := 1; s < 4; s++ {
		if st.Seats[s] == nil || !st.Seats[s].Bot || !st.Seats[s].Ready || !st.Seats[s].Connected {
			t.Fatalf("assento %d deveria ser bot pronto: %+v", s, st.Seats[s])
		}
		names[st.Seats[s].DisplayName] = true
	}
	if len(names) != 3 || st.Seats[0].Bot || !st.Seats[0].Ready {
		t.Fatalf("room_state = %+v", st)
	}
	players := h.store.last().Players
	for s := 1; s < 4; s++ {
		if !players[s].Bot {
			t.Fatalf("Create deveria receber bot no assento %d: %+v", s, players)
		}
	}
	if players[0].Bot {
		t.Fatal("o host não é bot")
	}
	for _, b := range testBots {
		if h.reg.wasSeated(b.UserID) || h.reg.wasReleased(b.UserID) {
			t.Fatalf("bot %s não pode passar pelo índice do lobby", b.UserID)
		}
		if len(h.out.all(b.UserID)) != 0 {
			t.Fatalf("nada deve ser enviado a bot %s", b.UserID)
		}
	}
	sum := h.reg.lastSummary()
	if sum.Room.Seats[1] == nil || !sum.Room.Seats[1].Bot {
		t.Fatalf("resumo deve marcar bot: %+v", sum.Room.Seats)
	}
}

func TestStartWithBotsWaitsForReadyHumans(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(1)
	startWithBots(h, "u0")
	if h.room.Status() != protocol.RoomWaiting {
		t.Fatal("não deveria começar com humano não pronto")
	}
	st := last[protocol.RoomStatePayload](t, h.out, "u1", protocol.MsgRoomState)
	if !st.Seats[0].Ready || st.Seats[1].Ready || !st.Seats[2].Bot || !st.Seats[3].Bot {
		t.Fatalf("room_state = %+v", st)
	}
	h.cmd("u1", protocol.TypeSetReady, protocol.SetReadyPayload{Ready: true})
	if h.room.Status() != protocol.RoomPlaying {
		t.Fatal("deveria começar quando o humano ficar pronto")
	}
}

func TestStartWithBotsErrors(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(1)
	startWithBots(h, "u1")
	if lastCode(t, h.out, "u1") != protocol.CodeNotHost {
		t.Fatalf("códigos = %v", h.out.errorCodes("u1"))
	}
	h.join(2)
	h.join(3)
	startWithBots(h, "u0")
	if lastCode(t, h.out, "u0") != protocol.CodeNoEmptySeat {
		t.Fatalf("códigos = %v", h.out.errorCodes("u0"))
	}
	h.readyAll()
	if h.room.Status() != protocol.RoomPlaying {
		t.Fatal("deveria estar jogando")
	}
	startWithBots(h, "u0")
	if lastCode(t, h.out, "u0") != protocol.CodeNotWaiting {
		t.Fatalf("códigos = %v", h.out.errorCodes("u0"))
	}
}

func TestStartWithBotsRosterTooSmall(t *testing.T) {
	h := newHarness(t, func(o *room.Options) { o.Bots = testBots[:2] })
	h.join(0)
	startWithBots(h, "u0")
	if lastCode(t, h.out, "u0") != protocol.CodeInternal || h.room.Status() != protocol.RoomWaiting {
		t.Fatal("roster insuficiente deve falhar sem sentar ninguém")
	}
	st := last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState)
	if st.Seats[1] != nil {
		t.Fatalf("nenhum bot deveria ter sentado: %+v", st.Seats)
	}
}

func TestRematchKeepsBotsReady(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	startWithBots(h, "u0")
	playToEnd(h, map[engine.Seat]string{0: "u0", 1: "b0", 2: "b1", 3: "b2"})
	if h.room.Status() != protocol.RoomWaiting {
		t.Fatalf("a partida deveria ter acabado: %s", h.room.Status())
	}
	st := last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState)
	if st.Seats[0].Ready || !st.Seats[1].Ready || !st.Seats[2].Ready || !st.Seats[3].Ready || !st.Seats[1].Bot {
		t.Fatalf("room_state = %+v", st)
	}
	h.cmd("u0", protocol.TypeSetReady, protocol.SetReadyPayload{Ready: true})
	if h.room.Status() != protocol.RoomPlaying || len(h.store.order) != 2 {
		t.Fatal("a revanche deveria começar sozinha")
	}
}

func TestTakeSeatReplacesBot(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(1)
	startWithBots(h, "u0") // bots em 2 e 3; u1 não pronto
	two := engine.Seat(2)
	h.send(room.Join{UserID: "u2", DisplayName: "Caio", Seat: &two, Seq: 1}) // cadeira de bot, sala sem vaga vazia
	st := last[protocol.RoomStatePayload](t, h.out, "u2", protocol.MsgRoomState)
	if st.YourSeat != 2 || st.Seats[2].Bot || st.Seats[2].Ready || !st.Seats[3].Bot {
		t.Fatalf("room_state = %+v", st)
	}
	if !h.reg.wasSeated("u2") || h.reg.wasReleased("b0") || h.reg.wasReleased("b1") {
		t.Fatal("só o humano entra no índice")
	}
	// Join sem assento, sala sem vaga vazia mas com bot: ocupa o bot de menor assento (o 3).
	h.send(room.Join{UserID: "u3", DisplayName: "Duda", Seq: 2})
	st = last[protocol.RoomStatePayload](t, h.out, "u3", protocol.MsgRoomState)
	if st.YourSeat != 3 {
		t.Fatalf("deveria ocupar o bot restante: %+v", st)
	}
	// Agora 4 humanos: room_full.
	h.send(room.Join{UserID: "u9", DisplayName: "Eva", Seq: 3})
	if lastCode(t, h.out, "u9") != protocol.CodeRoomFull || !h.reg.wasReleased("u9") {
		t.Fatal("sala de 4 humanos deve recusar")
	}
}

func TestJoinWithoutSeatTakesLowestBotSeat(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(1)
	startWithBots(h, "u0") // bots em 2 e 3
	h.send(room.Join{UserID: "u2", DisplayName: "Caio", Seq: 1})
	st := last[protocol.RoomStatePayload](t, h.out, "u2", protocol.MsgRoomState)
	if st.YourSeat != 2 || !st.Seats[3].Bot {
		t.Fatalf("deveria ocupar o bot de menor assento: %+v", st)
	}
}

func TestTakeSeatCommandReplacesBot(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(1)
	startWithBots(h, "u0")
	h.cmd("u1", protocol.TypeSetReady, protocol.SetReadyPayload{Ready: false})
	h.cmd("u1", protocol.TypeTakeSeat, protocol.TakeSeatPayload{Seat: 3})
	st := last[protocol.RoomStatePayload](t, h.out, "u1", protocol.MsgRoomState)
	if st.YourSeat != 3 || st.Seats[3].Bot || st.Seats[1] != nil || !st.Seats[2].Bot {
		t.Fatalf("room_state = %+v", st)
	}
}

func TestInviteIntoBotSeatTakesIt(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(1)
	startWithBots(h, "u0")
	two := engine.Seat(2)
	h.send(room.Join{UserID: "u2", DisplayName: "Caio", Seat: &two, Seq: 1, Invite: true})
	st := last[protocol.RoomStatePayload](t, h.out, "u2", protocol.MsgRoomState)
	if st.YourSeat != 2 {
		t.Fatalf("convite no assento do bot deve sentar nele: %+v", st)
	}
}

func TestHostKicksBotWithoutConfirmation(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(1)
	startWithBots(h, "u0")
	h.cmd("u1", protocol.TypeKick, protocol.KickPayload{Seat: 2})
	if lastCode(t, h.out, "u1") != protocol.CodeNotHost {
		t.Fatal("só o host remove")
	}
	h.cmd("u0", protocol.TypeKick, protocol.KickPayload{Seat: 2})
	st := last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState)
	if st.Seats[2] != nil || !st.Seats[3].Bot {
		t.Fatalf("room_state = %+v", st)
	}
	if h.reg.wasReleased("b0") || h.reg.wasReleased("b1") || h.reg.wasReleased("b2") {
		t.Fatal("bot não passa por Released")
	}
	// recompleta com outro bot, nome distinto do que ficou
	startWithBots(h, "u0")
	st = last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState)
	if st.Seats[2] == nil || !st.Seats[2].Bot || st.Seats[2].DisplayName == st.Seats[3].DisplayName {
		t.Fatalf("room_state = %+v", st)
	}
}

func TestLastHumanLeavingDestroysRoomWithBots(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(1)
	startWithBots(h, "u0")
	h.cmd("u1", protocol.TypeLeaveRoom, nil)
	if h.reg.destroyed != 0 || h.room.Closed() {
		t.Fatal("ainda há humano")
	}
	h.cmd("u0", protocol.TypeLeaveRoom, nil)
	if h.reg.destroyed != 1 || !h.room.Closed() {
		t.Fatal("a sala deveria ser destruída com só bots")
	}
	for _, b := range testBots {
		if h.reg.wasReleased(b.UserID) {
			t.Fatalf("bot %s não passa por Released", b.UserID)
		}
	}
}

func TestHostPassesToHumanNotBot(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	three := engine.Seat(3)
	h.send(room.Join{UserID: "u1", DisplayName: "Bia", Seat: &three, Seq: 1})
	startWithBots(h, "u0") // bots em 1 e 2
	h.cmd("u0", protocol.TypeLeaveRoom, nil)
	st := last[protocol.RoomStatePayload](t, h.out, "u1", protocol.MsgRoomState)
	if st.Host != 3 || st.Seats[0] != nil {
		t.Fatalf("o novo host deveria ser o humano do assento 3: %+v", st)
	}
	if h.room.Closed() {
		t.Fatal("a sala continua com um humano")
	}
}

func TestBotSeatNeverRemovedForDisconnect(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(1)
	startWithBots(h, "u0")
	h.send(room.Disconnected{UserID: "u1"})
	h.advance(testTiming.WaitingDisconnect + 2)
	st := last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState)
	if st.Seats[1] != nil || st.Seats[2] == nil || !st.Seats[2].Bot || !st.Seats[3].Bot || !st.Seats[2].Connected {
		t.Fatalf("room_state = %+v", st)
	}
}

// playToEnd joga a partida inteira em nome de cada assento (os bots ainda não jogam sozinhos nesta tarefa).
func playToEnd(h *harness, userOf map[engine.Seat]string) {
	h.t.Helper()
	for range 5000 {
		if h.room.Status() != protocol.RoomPlaying {
			return
		}
		g := h.room.Game()
		if g.Phase == engine.PhaseHandEnd {
			h.advance(testTiming.HandPause)
			continue
		}
		a, ok := nextMove(g)
		if !ok {
			h.t.Fatalf("nenhum assento pendente na fase %s", g.Phase)
		}
		h.send(room.Command{UserID: userOf[a.Seat], Message: clientMsg(a)})
	}
	h.t.Fatal("a partida não terminou")
}
