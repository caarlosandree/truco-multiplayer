package lobby

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

type stubClient struct {
	id   string
	mu   sync.Mutex
	msgs []protocol.ServerMessage
}

func (c *stubClient) UserID() string { return c.id }
func (c *stubClient) Send(m protocol.ServerMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, m)
}
func (c *stubClient) Close(CloseReason) {}
func (c *stubClient) has(typ protocol.ServerMessageType, ok func(protocol.ServerMessage) bool) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, m := range c.msgs {
		if m.Type == typ && (ok == nil || ok(m)) {
			return true
		}
	}
	return false
}

type stubStore struct{}

func (stubStore) Create(context.Context, uint64, engine.Seat, [4]replay.PlayerSeat) (string, error) {
	return "m", nil
}
func (stubStore) AppendAction(context.Context, string, int, engine.Action, replay.Source) error {
	return nil
}
func (stubStore) Finish(context.Context, string, bool) error    { return nil }
func (stubStore) Abandon(context.Context, string, string) error { return nil }

type onePerson struct{ who Invitee }

func (p onePerson) Invitee(context.Context, string) (Invitee, error) { return p.who, nil }
func (onePerson) MailInvite(string, string, string, string)          {}

// Determinístico: deixa o usuário "reservado, mas ainda não sentado" (entrada por código em voo) e confere que o
// aceite não marca "aceitou" antes de sentar e que sempre termina em resultado observável.
func TestAcceptWithReservedButUnseatedUser(t *testing.T) {
	setup := func(t *testing.T) (*Lobby, *stubClient, *stubClient, string, string) {
		l := New(context.Background(), Options{Timing: room.Timing{Play: time.Minute, Vote: time.Minute, Choice: time.Minute,
			Burn: time.Minute, Absent: time.Minute, HandPause: time.Second, AbsenceGrace: time.Minute, AbsenceBudget: time.Minute,
			WaitingDisconnect: time.Minute}, MaxRooms: 3, Store: stubStore{}, Clock: clock.NewManual(time.Unix(0, 0)),
			Rand: rand.NewChaCha8([32]byte{5}), Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
			People: onePerson{Invitee{ID: "u1", Username: "bia", DisplayName: "Bia"}}})
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = l.Shutdown(ctx)
		})
		ana, bia := &stubClient{id: "u0"}, &stubClient{id: "u1"}
		l.Connect(ana, protocol.UserInfo{ID: "u0", Username: "ana", DisplayName: "Ana"})
		l.Connect(bia, protocol.UserInfo{ID: "u1", Username: "bia", DisplayName: "Bia"})
		l.Dispatch(ana, protocol.ClientMessage{Type: protocol.TypeCreateRoom, Seq: 1, Payload: json.RawMessage(`{"visibility":"private"}`)})
		deadline := time.Now().Add(2 * time.Second)
		for !ana.has(protocol.MsgRoomState, nil) && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		l.Dispatch(ana, protocol.ClientMessage{Type: protocol.TypeInvite, Seq: 2, Payload: json.RawMessage(`{"username":"bia"}`)})
		var roomID, inviteID string
		l.mu.Lock()
		for id := range l.rooms {
			roomID = id
		}
		for id := range l.invites {
			inviteID = id
		}
		l.index["u1"] = roomID // a entrada por código de Bia está em voo: reservou, a sala ainda não respondeu
		l.mu.Unlock()
		return l, ana, bia, roomID, inviteID
	}
	respond := func(l *Lobby, c *stubClient, id string, accept bool) {
		raw, _ := json.Marshal(protocol.InviteRespondPayload{InviteID: id, Accept: accept})
		l.Dispatch(c, protocol.ClientMessage{Type: protocol.TypeInviteRespond, Seq: 9, Payload: raw})
	}
	stateIs := func(l *Lobby, id string, st protocol.InviteState) bool {
		l.mu.Lock()
		defer l.mu.Unlock()
		inv := l.invites[id]
		return inv != nil && inv.state == st
	}

	t.Run("aceite não marca aceitou antes de sentar; a entrada falha e responde com o seq do aceite", func(t *testing.T) {
		l, _, bia, roomID, id := setup(t)
		respond(l, bia, id, true)
		if !stateIs(l, id, protocol.InvitePending) {
			t.Fatal("convite não pode virar aceito antes de a pessoa sentar")
		}
		l.Released(roomID, "u1") // a sala recusou a entrada por código
		if !bia.has(protocol.MsgError, func(m protocol.ServerMessage) bool {
			p := m.Payload.(protocol.ErrorPayload)
			return p.Seq == 9 && p.Code == protocol.CodeInviteExpired
		}) {
			t.Fatal("o aceite deve terminar em erro com o seq dele")
		}
		l.mu.Lock()
		defer l.mu.Unlock()
		if l.invites[id] != nil || l.index["u1"] != "" {
			t.Fatal("convite deve expirar e a reserva ser liberada")
		}
	})

	t.Run("a entrada dá certo: Seated marca aceitou", func(t *testing.T) {
		l, _, bia, roomID, id := setup(t)
		respond(l, bia, id, true)
		l.Seated(roomID, "u1")
		if !stateIs(l, id, protocol.InviteAccepted) {
			t.Fatal("Seated deve marcar aceitou")
		}
	})

	t.Run("recusar durante o aceite é ignorado", func(t *testing.T) {
		l, _, bia, roomID, id := setup(t)
		respond(l, bia, id, true)
		respond(l, bia, id, false)
		if !stateIs(l, id, protocol.InvitePending) {
			t.Fatal("recusa com aceite em andamento deve ser ignorada")
		}
		l.Seated(roomID, "u1")
		if !stateIs(l, id, protocol.InviteAccepted) {
			t.Fatal("deveria ficar aceito")
		}
	})

	t.Run("a sala alvo fecha com o aceite em voo: erro com o seq do aceite", func(t *testing.T) {
		l, _, bia, roomID, id := setup(t)
		respond(l, bia, id, true)
		l.Destroyed(roomID)
		if !bia.has(protocol.MsgError, func(m protocol.ServerMessage) bool {
			p := m.Payload.(protocol.ErrorPayload)
			return p.Seq == 9 && p.Code == protocol.CodeInviteExpired
		}) {
			t.Fatal("aceite sem resposta quando a sala fechou")
		}
		l.mu.Lock()
		defer l.mu.Unlock()
		if _, still := l.accepting["u1"]; still || l.index["u1"] != "" {
			t.Fatal("aceite e reserva devem ser liberados")
		}
	})
}

func TestFreeSeatsCountsBotSeats(t *testing.T) {
	var rs protocol.RoomSummary
	rs.Seats[0] = &protocol.SeatSummary{DisplayName: "Ana"}
	rs.Seats[1] = &protocol.SeatSummary{DisplayName: "Seu Zé", Bot: true}
	rs.Seats[2] = &protocol.SeatSummary{DisplayName: "Bia"}
	if got := freeSeats(room.Summary{Room: rs}); got != 2 { // 1 nil + 1 bot
		t.Fatalf("freeSeats = %d, esperado 2", got)
	}
	rs.Seats[3] = &protocol.SeatSummary{DisplayName: "Caio"}
	if got := freeSeats(room.Summary{Room: rs}); got != 1 {
		t.Fatalf("freeSeats = %d, esperado 1", got)
	}
}
