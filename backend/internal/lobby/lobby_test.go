package lobby_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/enginetest"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/lobby"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

var timing = room.Timing{Play: 30 * time.Second, Vote: 20 * time.Second, Choice: 15 * time.Second, Burn: 15 * time.Second,
	Absent: 5 * time.Second, HandPause: 4 * time.Second, AbsenceGrace: 2 * time.Minute, AbsenceBudget: 5 * time.Minute,
	WaitingDisconnect: time.Minute}

type fakeClient struct {
	id     string
	mu     sync.Mutex
	msgs   []protocol.ServerMessage
	closed lobby.CloseReason
}

func (c *fakeClient) UserID() string { return c.id }

func (c *fakeClient) Send(m protocol.ServerMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, m)
}

func (c *fakeClient) Close(r lobby.CloseReason) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = r
}

func (c *fakeClient) messages() []protocol.ServerMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]protocol.ServerMessage(nil), c.msgs...)
}

func (c *fakeClient) closeReason() lobby.CloseReason {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *fakeClient) count(typ protocol.ServerMessageType) int {
	n := 0
	for _, m := range c.messages() {
		if m.Type == typ {
			n++
		}
	}
	return n
}

// waitFor espera (até 2s) uma mensagem do tipo que satisfaça match (nil = qualquer) e a devolve.
func (c *fakeClient) waitFor(t *testing.T, typ protocol.ServerMessageType, match func(protocol.ServerMessage) bool) protocol.ServerMessage {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, m := range c.messages() {
			if m.Type == typ && (match == nil || match(m)) {
				return m
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("%s não recebeu %s", c.id, typ)
	return protocol.ServerMessage{}
}

func errorCode(code string) func(protocol.ServerMessage) bool {
	return func(m protocol.ServerMessage) bool { return m.Payload.(protocol.ErrorPayload).Code == code }
}

func roomState(t *testing.T, c *fakeClient) protocol.RoomStatePayload {
	t.Helper()
	return c.waitFor(t, protocol.MsgRoomState, nil).Payload.(protocol.RoomStatePayload)
}

type nopStore struct {
	mu sync.Mutex
	n  int
}

func (s *nopStore) Create(context.Context, uint64, engine.Seat, [4]replay.PlayerSeat) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.n++
	return fmt.Sprintf("m%d", s.n), nil
}
func (s *nopStore) AppendAction(context.Context, string, int, engine.Action, replay.Source) error {
	return nil
}
func (s *nopStore) Finish(context.Context, string) error          { return nil }
func (s *nopStore) Abandon(context.Context, string, string) error { return nil }

func newLobby(t *testing.T, maxRooms int) *lobby.Lobby {
	l := lobby.New(context.Background(), lobby.Options{Timing: timing, MaxRooms: maxRooms, Store: &nopStore{},
		Clock: clock.NewManual(time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)), Rand: rand.NewChaCha8([32]byte{2}),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = l.Shutdown(ctx)
	})
	return l
}

func connect(l *lobby.Lobby, id, name string) *fakeClient {
	c := &fakeClient{id: id}
	l.Connect(c, protocol.UserInfo{ID: id, Username: strings.ToLower(name), DisplayName: name})
	return c
}

func dispatch(l *lobby.Lobby, c *fakeClient, typ string, payload any) {
	msg := protocol.ClientMessage{Type: typ, Seq: 1}
	if payload != nil {
		msg.Payload, _ = json.Marshal(payload)
	}
	l.Dispatch(c, msg)
}

func TestConnectSendsHelloAndSnapshot(t *testing.T) {
	l := newLobby(t, 3)
	c := connect(l, "u0", "Ana")
	msgs := c.messages()
	if len(msgs) != 2 || msgs[0].Type != protocol.MsgHello || msgs[1].Type != protocol.MsgLobbySnapshot {
		t.Fatalf("mensagens = %+v", msgs)
	}
	if hello := msgs[0].Payload.(protocol.HelloPayload); hello.User.ID != "u0" || hello.ProtocolVersion != protocol.ProtocolVersion {
		t.Fatalf("hello = %+v", hello)
	}
}

func TestPublicRoomIsListedAndPrivateIsNot(t *testing.T) {
	l := newLobby(t, 3)
	ana, bia, caio := connect(l, "u0", "Ana"), connect(l, "u1", "Bia"), connect(l, "u2", "Caio")
	dispatch(l, ana, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic})
	up := bia.waitFor(t, protocol.MsgRoomUpserted, nil).Payload.(protocol.RoomUpsertedPayload)
	if up.Room.Name != "Sala de Ana" || up.Room.Seats[0] == nil || up.Room.Seats[0].DisplayName != "Ana" {
		t.Fatalf("room_upserted = %+v", up)
	}
	dispatch(l, caio, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPrivate})
	roomState(t, caio)
	late := connect(l, "u3", "Duda")
	snap := late.waitFor(t, protocol.MsgLobbySnapshot, nil).Payload.(protocol.LobbySnapshotPayload)
	if len(snap.Rooms) != 1 || snap.Rooms[0].ID != up.Room.ID {
		t.Fatalf("snapshot deve listar só a sala pública: %+v", snap.Rooms)
	}
}

func TestJoinByCodeIsCaseAndSpaceInsensitive(t *testing.T) {
	l := newLobby(t, 3)
	ana, bia := connect(l, "u0", "Ana"), connect(l, "u1", "Bia")
	dispatch(l, ana, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPrivate})
	code := roomState(t, ana).Code
	typed := " " + strings.ToLower(code) + " "
	dispatch(l, bia, protocol.TypeJoinRoom, protocol.JoinRoomPayload{Code: &typed})
	if st := roomState(t, bia); st.YourSeat != 1 || st.Code != code {
		t.Fatalf("room_state = %+v", st)
	}
}

func TestPrivateRoomIsNotJoinableByID(t *testing.T) {
	l := newLobby(t, 3)
	ana, bia := connect(l, "u0", "Ana"), connect(l, "u1", "Bia")
	dispatch(l, ana, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPrivate})
	id := roomState(t, ana).ID
	dispatch(l, bia, protocol.TypeJoinRoom, protocol.JoinRoomPayload{RoomID: &id})
	bia.waitFor(t, protocol.MsgError, errorCode(protocol.CodeRoomNotFound))
}

func TestOneRoomPerUserAndMaxRooms(t *testing.T) {
	l := newLobby(t, 2)
	ana, bia, caio := connect(l, "u0", "Ana"), connect(l, "u1", "Bia"), connect(l, "u2", "Caio")
	public := protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic}
	dispatch(l, ana, protocol.TypeCreateRoom, public)
	roomState(t, ana)
	dispatch(l, ana, protocol.TypeCreateRoom, public)
	ana.waitFor(t, protocol.MsgError, errorCode(protocol.CodeAlreadyInRoom))
	dispatch(l, bia, protocol.TypeCreateRoom, public)
	roomState(t, bia)
	dispatch(l, caio, protocol.TypeCreateRoom, public)
	caio.waitFor(t, protocol.MsgError, errorCode(protocol.CodeTooManyRooms))
}

func TestLeavingRoomReturnsToLobby(t *testing.T) {
	l := newLobby(t, 3)
	ana, bia := connect(l, "u0", "Ana"), connect(l, "u1", "Bia")
	dispatch(l, ana, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic})
	id := roomState(t, ana).ID
	dispatch(l, bia, protocol.TypeJoinRoom, protocol.JoinRoomPayload{RoomID: &id})
	roomState(t, bia)
	before := bia.count(protocol.MsgLobbySnapshot)
	dispatch(l, bia, protocol.TypeLeaveRoom, nil)
	bia.waitFor(t, protocol.MsgLobbySnapshot, func(protocol.ServerMessage) bool { return bia.count(protocol.MsgLobbySnapshot) > before })
	msgs := bia.messages()
	leftAt, snapAt := -1, -1
	for i, m := range msgs {
		if m.Type == protocol.MsgLeftRoom {
			leftAt = i
		}
		if m.Type == protocol.MsgLobbySnapshot {
			snapAt = i
		}
	}
	if leftAt < 0 || snapAt < leftAt {
		t.Fatalf("left_room deve vir antes do novo lobby_snapshot: %d, %d", leftAt, snapAt)
	}
}

func TestDestroyedRoomLeavesTheList(t *testing.T) {
	l := newLobby(t, 3)
	ana, bia := connect(l, "u0", "Ana"), connect(l, "u1", "Bia")
	dispatch(l, ana, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic})
	id := roomState(t, ana).ID
	bia.waitFor(t, protocol.MsgRoomUpserted, nil)
	dispatch(l, ana, protocol.TypeLeaveRoom, nil)
	bia.waitFor(t, protocol.MsgRoomRemoved, func(m protocol.ServerMessage) bool {
		return m.Payload.(protocol.RoomRemovedPayload).RoomID == id
	})
	dispatch(l, bia, protocol.TypeJoinRoom, protocol.JoinRoomPayload{RoomID: &id})
	bia.waitFor(t, protocol.MsgError, errorCode(protocol.CodeRoomNotFound))
}

func TestStaleConnectionIsIgnored(t *testing.T) {
	l := newLobby(t, 3)
	old := connect(l, "u0", "Ana")
	fresh := connect(l, "u0", "Ana")
	if old.closeReason() != lobby.CloseReplaced {
		t.Fatal("a conexão antiga deve ser fechada como substituída")
	}
	fresh.waitFor(t, protocol.MsgHello, nil)
	dispatch(l, old, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic})
	dispatch(l, fresh, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic})
	if st := roomState(t, fresh); st.YourSeat != 0 {
		t.Fatalf("room_state = %+v", st)
	}
	if fresh.count(protocol.MsgError) != 0 {
		t.Fatal("a mensagem da conexão antiga não pode ter criado sala")
	}
}

func TestUnknownTypeAndCommandOutsideRoom(t *testing.T) {
	l := newLobby(t, 3)
	ana := connect(l, "u0", "Ana")
	dispatch(l, ana, "hack", nil)
	ana.waitFor(t, protocol.MsgError, errorCode(protocol.CodeBadRequest))
	dispatch(l, ana, protocol.TypeSetReady, protocol.SetReadyPayload{Ready: true})
	ana.waitFor(t, protocol.MsgError, errorCode(protocol.CodeNotInRoom))
}

func TestRestoreIndexesPlayersAndAbortEndsMatch(t *testing.T) {
	l := newLobby(t, 3)
	rec, _ := enginetest.RandomRecord(4)
	rec.Actions = rec.Actions[:12]
	state, err := replay.Rebuild(rec)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Restore([]match.Recovered{{Record: rec, State: state, NextPly: 12}}); err != nil {
		t.Fatal(err)
	}
	ana := connect(l, "u0", "Ana")
	if st := roomState(t, ana); st.Status != protocol.RoomPlaying || st.MatchID != rec.ID {
		t.Fatalf("room_state = %+v", st)
	}
	ana.waitFor(t, protocol.MsgState, nil)
	if ana.count(protocol.MsgLobbySnapshot) != 0 {
		t.Fatal("jogador de partida restaurada vai direto para a sala")
	}
	late := connect(l, "outsider", "Zeca")
	snap := late.waitFor(t, protocol.MsgLobbySnapshot, nil).Payload.(protocol.LobbySnapshotPayload)
	if len(snap.Rooms) != 1 || snap.Rooms[0].Score == nil {
		t.Fatalf("snapshot = %+v", snap.Rooms)
	}
	for i, s := range snap.Rooms[0].Seats {
		if s == nil || s.DisplayName == "" {
			t.Fatalf("assento %d vazio na sala restaurada", i)
		}
	}
	if err := l.Abort(context.Background(), rec.ID); err != nil {
		t.Fatal(err)
	}
	ended := ana.waitFor(t, protocol.MsgMatchEnded, nil).Payload.(protocol.MatchEndedPayload)
	if ended.Reason != protocol.EndAborted {
		t.Fatalf("match_ended = %+v", ended)
	}
	if err := l.Abort(context.Background(), "inexistente"); err != nil {
		t.Fatal("abort de partida sem sala não é erro")
	}
}

func TestShutdownClosesClientsAndRejectsConnect(t *testing.T) {
	l := newLobby(t, 3)
	ana := connect(l, "u0", "Ana")
	dispatch(l, ana, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic})
	roomState(t, ana)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := l.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if ana.closeReason() != lobby.CloseShutdown {
		t.Fatal("clientes fecham no desligamento")
	}
	if l.Connect(&fakeClient{id: "u9"}, protocol.UserInfo{ID: "u9"}) {
		t.Fatal("Connect deve recusar durante o desligamento")
	}
}

func TestDestroyedSendsSnapshotToReservedUsers(t *testing.T) {
	l := newLobby(t, 3)
	ana, bia := connect(l, "u0", "Ana"), connect(l, "u1", "Bia")
	dispatch(l, ana, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic})
	id := roomState(t, ana).ID
	before := bia.count(protocol.MsgLobbySnapshot)
	dispatch(l, bia, protocol.TypeJoinRoom, protocol.JoinRoomPayload{RoomID: &id})
	l.Destroyed(id) // a sala some com a reserva de Bia ainda no índice
	bia.waitFor(t, protocol.MsgLobbySnapshot, func(m protocol.ServerMessage) bool {
		return bia.count(protocol.MsgLobbySnapshot) > before && len(m.Payload.(protocol.LobbySnapshotPayload).Rooms) == 0
	})
}
