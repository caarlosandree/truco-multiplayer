package room_test

import (
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

func TestFirstJoinBecomesHostAndPublishes(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	st := last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState)
	if st.Host != 0 || st.YourSeat != 0 || st.Seats[0] == nil || st.Seats[0].DisplayName != "Ana" || !st.Seats[0].Connected ||
		st.Code != "K7QM2P" || st.Status != protocol.RoomWaiting || st.Visibility != protocol.VisibilityPublic || st.Name != "Sala de Ana" {
		t.Fatalf("room_state = %+v", st)
	}
	sum := h.reg.lastSummary()
	if !sum.Public || sum.Code != "K7QM2P" || sum.Room.ID != "r1" || sum.Room.Seats[0].DisplayName != "Ana" || sum.Room.Seats[1] != nil {
		t.Fatalf("resumo = %+v", sum)
	}
}

func TestJoinPicksRequestedOrFirstFreeSeat(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	three := engine.Seat(3)
	h.send(room.Join{UserID: "u1", DisplayName: "Bia", Seat: &three, Seq: 4})
	h.join(2)
	st := last[protocol.RoomStatePayload](t, h.out, "u2", protocol.MsgRoomState)
	if st.YourSeat != 1 || st.Seats[3] == nil || st.Seats[3].UserID != "u1" {
		t.Fatalf("room_state = %+v", st)
	}
	h.send(room.Join{UserID: "u3", DisplayName: "Duda", Seat: &three, Seq: 5})
	if lastCode(t, h.out, "u3") != protocol.CodeSeatTaken || !h.reg.wasReleased("u3") {
		t.Fatal("assento ocupado deve recusar e liberar o usuário no lobby")
	}
}

func TestJoinFullRoomReleasesUser(t *testing.T) {
	h := newHarness(t)
	h.seatAll()
	h.send(room.Join{UserID: "u9", DisplayName: "Eva", Seq: 9})
	if lastCode(t, h.out, "u9") != protocol.CodeRoomFull || !h.reg.wasReleased("u9") {
		t.Fatal("sala cheia deve recusar e liberar o usuário")
	}
}

func TestJoinIsIdempotentForMember(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(0)
	st := last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState)
	if st.Seats[1] != nil || h.out.count("u0", protocol.MsgRoomState) != 2 {
		t.Fatalf("segunda entrada deve só reenviar o estado: %+v", st)
	}
}

func TestTakeSeatMovesAndClearsReady(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(1)
	h.cmd("u1", protocol.TypeSetReady, protocol.SetReadyPayload{Ready: true})
	h.cmd("u1", protocol.TypeTakeSeat, protocol.TakeSeatPayload{Seat: 3})
	st := last[protocol.RoomStatePayload](t, h.out, "u1", protocol.MsgRoomState)
	if st.YourSeat != 3 || st.Seats[1] != nil || st.Seats[3].Ready {
		t.Fatalf("room_state = %+v", st)
	}
	h.cmd("u1", protocol.TypeTakeSeat, protocol.TakeSeatPayload{Seat: 0})
	if lastCode(t, h.out, "u1") != protocol.CodeSeatTaken {
		t.Fatal("trocar para assento ocupado deve falhar")
	}
}

func TestKickOnlyByHost(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(1)
	h.cmd("u1", protocol.TypeKick, protocol.KickPayload{Seat: 0})
	if lastCode(t, h.out, "u1") != protocol.CodeNotHost {
		t.Fatal("só o anfitrião expulsa")
	}
	h.cmd("u0", protocol.TypeKick, protocol.KickPayload{Seat: 1})
	if left := last[protocol.LeftRoomPayload](t, h.out, "u1", protocol.MsgLeftRoom); left.Reason != protocol.LeftKicked {
		t.Fatalf("left_room = %+v", left)
	}
	if !h.reg.wasReleased("u1") || last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState).Seats[1] != nil {
		t.Fatal("expulso deve sair do assento e do índice")
	}
}

func TestHostLeavingPassesHostToNextOccupant(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(1)
	h.join(2)
	h.cmd("u0", protocol.TypeLeaveRoom, nil)
	if st := last[protocol.RoomStatePayload](t, h.out, "u1", protocol.MsgRoomState); st.Host != 1 || st.Seats[0] != nil {
		t.Fatalf("room_state = %+v", st)
	}
	if last[protocol.LeftRoomPayload](t, h.out, "u0", protocol.MsgLeftRoom).Reason != protocol.LeftLeft {
		t.Fatal("quem sai recebe left_room{left}")
	}
}

func TestLastLeaveDestroysRoom(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.cmd("u0", protocol.TypeLeaveRoom, nil)
	if h.reg.destroyed != 1 || !h.room.Closed() {
		t.Fatal("sala vazia deve ser destruída")
	}
	h.send(room.Join{UserID: "u1", DisplayName: "Bia", Seq: 2})
	if lastCode(t, h.out, "u1") != protocol.CodeRoomNotFound || !h.reg.wasReleased("u1") {
		t.Fatal("sala destruída recusa entradas e libera o usuário")
	}
}

func TestCommandsAreValidated(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.cmd("u9", protocol.TypeSetReady, protocol.SetReadyPayload{Ready: true})
	if lastCode(t, h.out, "u9") != protocol.CodeNotInRoom {
		t.Fatal("não membro recebe not_in_room")
	}
	h.cmd("u0", protocol.TypeTakeSeat, map[string]int{"seat": 9})
	if lastCode(t, h.out, "u0") != protocol.CodeBadRequest {
		t.Fatal("assento fora de 0..3 é bad_request")
	}
	h.cmd("u0", protocol.TypeSetReady, nil)
	if lastCode(t, h.out, "u0") != protocol.CodeBadRequest {
		t.Fatal("payload ausente é bad_request")
	}
}

func TestDisconnectAndReconnectUpdateSeat(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.join(1)
	h.send(room.Disconnected{UserID: "u1"})
	if last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState).Seats[1].Connected {
		t.Fatal("desconexão deve aparecer no room_state")
	}
	h.send(room.Connected{UserID: "u1"})
	if !last[protocol.RoomStatePayload](t, h.out, "u0", protocol.MsgRoomState).Seats[1].Connected {
		t.Fatal("reconexão deve aparecer no room_state")
	}
}
