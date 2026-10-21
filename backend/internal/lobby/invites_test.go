package lobby_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/lobby"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
)

type mailCall struct{ inviteeID, inviter, room, token string }

// fakePeople é o cadastro de contas dos testes; hold segura a consulta de um username até o teste soltar.
type fakePeople struct {
	mu    sync.Mutex
	users map[string]lobby.Invitee
	calls []mailCall
	holds map[string]chan struct{}
	asked chan string
}

func newPeople(users ...lobby.Invitee) *fakePeople {
	p := &fakePeople{users: map[string]lobby.Invitee{}, holds: map[string]chan struct{}{}, asked: make(chan string, 8)}
	for _, u := range users {
		p.users[u.Username] = u
	}
	return p
}

func (p *fakePeople) Invitee(ctx context.Context, username string) (lobby.Invitee, error) {
	p.mu.Lock()
	hold := p.holds[username]
	p.mu.Unlock()
	if hold != nil {
		p.asked <- username
		select {
		case <-hold:
		case <-ctx.Done():
			return lobby.Invitee{}, ctx.Err()
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if u, ok := p.users[username]; ok {
		return u, nil
	}
	return lobby.Invitee{}, lobby.ErrNoInvitee
}

func (p *fakePeople) MailInvite(inviteeID, inviterName, roomName, token string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, mailCall{inviteeID, inviterName, roomName, token})
}

func (p *fakePeople) mails() []mailCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]mailCall(nil), p.calls...)
}

func (p *fakePeople) forget(username string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.users, username)
}

// scene: Ana (u0) numa sala pública; Bia, Caio, Duda (u1..u3) e Edu (u4) online no lobby; Fabi (u5) só no cadastro.
type scene struct {
	l                         *lobby.Lobby
	people                    *fakePeople
	ana, bia, caio, duda, edu *fakeClient
	roomID                    string
}

func newScene(t *testing.T) *scene {
	t.Helper()
	people := newPeople(
		lobby.Invitee{ID: "u0", Username: "ana", DisplayName: "Ana"}, lobby.Invitee{ID: "u1", Username: "bia", DisplayName: "Bia"},
		lobby.Invitee{ID: "u2", Username: "caio", DisplayName: "Caio"}, lobby.Invitee{ID: "u3", Username: "duda", DisplayName: "Duda"},
		lobby.Invitee{ID: "u4", Username: "edu", DisplayName: "Edu"}, lobby.Invitee{ID: "u5", Username: "fabi", DisplayName: "Fabi"})
	l := newLobbyWith(t, 6, people)
	s := &scene{l: l, people: people, ana: connect(l, "u0", "Ana"), bia: connect(l, "u1", "Bia"), caio: connect(l, "u2", "Caio"),
		duda: connect(l, "u3", "Duda"), edu: connect(l, "u4", "Edu")}
	dispatch(l, s.ana, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic})
	s.roomID = roomState(t, s.ana).ID
	return s
}

func (s *scene) invite(c *fakeClient, username string, seat *engine.Seat) {
	dispatch(s.l, c, protocol.TypeInvite, protocol.InvitePayload{Username: username, Seat: seat})
}

func (s *scene) respond(c *fakeClient, id string, accept bool) {
	dispatch(s.l, c, protocol.TypeInviteRespond, protocol.InviteRespondPayload{InviteID: id, Accept: accept})
}

func (s *scene) joinByID(c *fakeClient) {
	id := s.roomID
	dispatch(s.l, c, protocol.TypeJoinRoom, protocol.JoinRoomPayload{RoomID: &id})
}

// waitSeats espera o resumo da sala no lobby (visto por quem está fora) ter n lugares ocupados:
// só então o Lobby já aplicou o Update e os convites enxergam a lotação certa.
func waitSeats(t *testing.T, watcher *fakeClient, roomID string, n int) {
	t.Helper()
	watcher.waitFor(t, protocol.MsgRoomUpserted, func(m protocol.ServerMessage) bool {
		r := m.Payload.(protocol.RoomUpsertedPayload).Room
		if r.ID != roomID {
			return false
		}
		got := 0
		for _, st := range r.Seats {
			if st != nil {
				got++
			}
		}
		return got == n
	})
}

func received(t *testing.T, c *fakeClient) protocol.InviteReceivedPayload {
	t.Helper()
	return c.waitFor(t, protocol.MsgInviteReceived, nil).Payload.(protocol.InviteReceivedPayload)
}

func waitInvites(t *testing.T, c *fakeClient, pred func([]protocol.RoomInvite) bool) []protocol.RoomInvite {
	t.Helper()
	m := c.waitFor(t, protocol.MsgRoomInvites, func(m protocol.ServerMessage) bool {
		return pred(m.Payload.(protocol.RoomInvitesPayload).Invites)
	})
	return m.Payload.(protocol.RoomInvitesPayload).Invites
}

func stateOf(list []protocol.RoomInvite, username string) (protocol.RoomInvite, bool) {
	for _, i := range list {
		if i.Username == username {
			return i, true
		}
	}
	return protocol.RoomInvite{}, false
}

func hasState(username string, st protocol.InviteState) func([]protocol.RoomInvite) bool {
	return func(l []protocol.RoomInvite) bool { i, ok := stateOf(l, username); return ok && i.State == st }
}

func expiredFor(id string) func(protocol.ServerMessage) bool {
	return func(m protocol.ServerMessage) bool { return m.Payload.(protocol.InviteExpiredPayload).InviteID == id }
}

func quiet() { time.Sleep(60 * time.Millisecond) }

func seatPtr(s engine.Seat) *engine.Seat { return &s }

func TestInviteValidation(t *testing.T) {
	s := newScene(t)
	s.invite(s.caio, "bia", nil)
	s.caio.waitFor(t, protocol.MsgError, errorCode(protocol.CodeNotInRoom))
	waitSeats(t, s.edu, s.roomID, 1)

	s.invite(s.ana, "ninguem", seatPtr(0)) // lugar ocupado vem antes de "convidado existe"
	s.ana.waitFor(t, protocol.MsgError, errorCode(protocol.CodeSeatTaken))
	s.invite(s.ana, "ninguem", nil)
	s.ana.waitFor(t, protocol.MsgError, errorCode(protocol.CodeUserNotFound))
	for _, ruim := range []string{"bi\x00a", "bi\xffa"} { // NUL e UTF-8 inválido: o mesmo que inexistente
		s.invite(s.ana, ruim, nil)
		s.ana.waitFor(t, protocol.MsgError, errorCode(protocol.CodeUserNotFound))
	}
	s.invite(s.ana, "ana", nil)
	s.ana.waitFor(t, protocol.MsgError, errorCode(protocol.CodeCannotInviteSelf))

	s.joinByID(s.bia)
	waitSeats(t, s.edu, s.roomID, 2)
	s.invite(s.ana, "bia", nil)
	s.ana.waitFor(t, protocol.MsgError, errorCode(protocol.CodeAlreadyInRoom))

	s.joinByID(s.caio)
	s.joinByID(s.duda)
	waitSeats(t, s.edu, s.roomID, 4)
	s.invite(s.ana, "ninguem", seatPtr(0))
	s.ana.waitFor(t, protocol.MsgError, errorCode(protocol.CodeRoomFull))

	for _, c := range []*fakeClient{s.ana, s.bia, s.caio, s.duda} {
		dispatch(s.l, c, protocol.TypeSetReady, protocol.SetReadyPayload{Ready: true})
	}
	s.edu.waitFor(t, protocol.MsgRoomUpserted, func(m protocol.ServerMessage) bool {
		return m.Payload.(protocol.RoomUpsertedPayload).Room.Status == protocol.RoomPlaying
	})
	s.invite(s.ana, "ninguem", nil)
	s.ana.waitFor(t, protocol.MsgError, errorCode(protocol.CodeMatchInProgress))
}

func TestInviteOnlineAndOffline(t *testing.T) {
	s := newScene(t)
	waitSeats(t, s.edu, s.roomID, 1)
	s.invite(s.ana, "bia", seatPtr(2))
	got := received(t, s.bia)
	if got.InviteID == "" || got.Room.ID != s.roomID || got.Room.Name != "Sala de Ana" || got.Room.Visibility != protocol.VisibilityPublic ||
		got.From.DisplayName != "Ana" || got.Seat == nil || *got.Seat != 2 || got.FreeSeats != 3 {
		t.Fatalf("invite_received = %+v", got)
	}
	if len(s.people.mails()) != 0 {
		t.Fatalf("convidado online não recebe e-mail: %+v", s.people.mails())
	}
	s.invite(s.ana, "fabi", nil)
	list := waitInvites(t, s.ana, func(l []protocol.RoomInvite) bool { i, ok := stateOf(l, "fabi"); return ok && i.Emailed })
	if i, _ := stateOf(list, "bia"); i.Emailed || i.State != protocol.InvitePending || i.DisplayName != "Bia" {
		t.Fatalf("lista = %+v", list)
	}
	mails := s.people.mails()
	if len(mails) != 1 || mails[0].inviteeID != "u5" || mails[0].inviter != "Ana" || mails[0].room != "Sala de Ana" || len(mails[0].token) < 40 {
		t.Fatalf("e-mails = %+v", mails)
	}
	// o token só vai no e-mail: nem a lista de quem convidou nem o aviso do convidado o carregam
	for _, c := range []*fakeClient{s.ana, s.bia} {
		raw, _ := json.Marshal(c.messages())
		if strings.Contains(string(raw), mails[0].token) {
			t.Fatalf("token vazou para %s", c.id)
		}
	}
}

func TestOnePendingPerInviteeAndRoom(t *testing.T) {
	s := newScene(t)
	waitSeats(t, s.edu, s.roomID, 1)
	s.invite(s.ana, "bia", nil)
	first := received(t, s.bia)
	s.invite(s.ana, "bia", nil)
	waitInvites(t, s.ana, func(l []protocol.RoomInvite) bool { return len(l) == 1 })
	quiet()
	if n := s.bia.count(protocol.MsgInviteReceived); n != 1 {
		t.Fatalf("convite repetido gerou novo aviso: %d", n)
	}
	s.respond(s.bia, first.InviteID, false)
	waitInvites(t, s.ana, hasState("bia", protocol.InviteDeclined))
	s.invite(s.ana, "bia", nil)
	s.bia.waitFor(t, protocol.MsgInviteReceived, func(m protocol.ServerMessage) bool {
		return m.Payload.(protocol.InviteReceivedPayload).InviteID != first.InviteID
	})
	if list := waitInvites(t, s.ana, hasState("bia", protocol.InvitePending)); len(list) != 1 {
		t.Fatalf("reconvite deve substituir a linha recusada: %+v", list)
	}

	// e-mail: no máximo um por pessoa e sala, mesmo depois de recusar
	s.invite(s.ana, "fabi", nil)
	s.invite(s.ana, "fabi", nil)
	fabi := connect(s.l, "u5", "Fabi")
	id := received(t, fabi).InviteID
	s.respond(fabi, id, false)
	waitInvites(t, s.ana, hasState("fabi", protocol.InviteDeclined))
	s.l.Disconnect(fabi)
	s.invite(s.ana, "fabi", nil)
	waitInvites(t, s.ana, hasState("fabi", protocol.InvitePending))
	if n := len(s.people.mails()); n != 1 {
		t.Fatalf("e-mails = %d, queria 1", n)
	}
}

func TestAcceptMovesFromWaitingRoom(t *testing.T) {
	s := newScene(t)
	dispatch(s.l, s.bia, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPrivate})
	old := roomState(t, s.bia).ID
	waitSeats(t, s.edu, s.roomID, 1)
	s.invite(s.ana, "bia", seatPtr(2))
	inv := received(t, s.bia)
	s.respond(s.bia, inv.InviteID, true)
	s.bia.waitFor(t, protocol.MsgLeftRoom, nil)
	st := s.bia.waitFor(t, protocol.MsgRoomState, func(m protocol.ServerMessage) bool {
		return m.Payload.(protocol.RoomStatePayload).ID == s.roomID
	}).Payload.(protocol.RoomStatePayload)
	if st.YourSeat != 2 || st.Seats[2] == nil || st.Seats[2].UserID != "u1" {
		t.Fatalf("Bia deveria sentar no lugar do convite: %+v", st)
	}
	waitInvites(t, s.ana, hasState("bia", protocol.InviteAccepted))
	s.bia.waitFor(t, protocol.MsgInviteExpired, expiredFor(inv.InviteID))
	// a sala antiga de Bia fechou
	dispatch(s.l, s.caio, protocol.TypeJoinRoom, protocol.JoinRoomPayload{RoomID: &old})
	s.caio.waitFor(t, protocol.MsgError, errorCode(protocol.CodeRoomNotFound))
}

func TestAcceptWhilePlayingIsRefused(t *testing.T) {
	s := newScene(t)
	waitSeats(t, s.edu, s.roomID, 1)
	gus, hugo, iris := connect(s.l, "u6", "Gus"), connect(s.l, "u7", "Hugo"), connect(s.l, "u8", "Iris")
	dispatch(s.l, s.bia, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic})
	other := roomState(t, s.bia).ID
	for _, c := range []*fakeClient{gus, hugo, iris} {
		dispatch(s.l, c, protocol.TypeJoinRoom, protocol.JoinRoomPayload{RoomID: &other})
	}
	for _, c := range []*fakeClient{s.bia, gus, hugo, iris} {
		dispatch(s.l, c, protocol.TypeSetReady, protocol.SetReadyPayload{Ready: true})
	}
	s.edu.waitFor(t, protocol.MsgRoomUpserted, func(m protocol.ServerMessage) bool {
		r := m.Payload.(protocol.RoomUpsertedPayload).Room
		return r.ID == other && r.Status == protocol.RoomPlaying
	})
	s.invite(s.ana, "bia", nil)
	inv := received(t, s.bia)
	s.respond(s.bia, inv.InviteID, true)
	s.bia.waitFor(t, protocol.MsgError, errorCode(protocol.CodeInviteePlaying))
	quiet()
	if i, _ := stateOf(waitInvites(t, s.ana, func(l []protocol.RoomInvite) bool { return len(l) > 0 }), "bia"); i.State != protocol.InvitePending {
		t.Fatalf("o convite deve seguir pendente: %+v", i)
	}
	if s.bia.count(protocol.MsgInviteExpired) != 0 {
		t.Fatal("recusa por partida não retira o aviso")
	}
}

func TestInviteExpiresOnFullStartAndClose(t *testing.T) {
	s := newScene(t)
	waitSeats(t, s.edu, s.roomID, 1)
	// sala enche -> pendentes expiram
	s.invite(s.ana, "edu", nil)
	inv := received(t, s.edu)
	s.joinByID(s.bia)
	s.joinByID(s.caio)
	s.joinByID(s.duda)
	s.edu.waitFor(t, protocol.MsgInviteExpired, expiredFor(inv.InviteID))
	waitInvites(t, s.ana, func(l []protocol.RoomInvite) bool { _, ok := stateOf(l, "edu"); return !ok })

	// sala fecha -> pendentes expiram
	s2 := newScene(t)
	waitSeats(t, s2.edu, s2.roomID, 1)
	s2.invite(s2.ana, "bia", nil)
	inv2 := received(t, s2.bia)
	dispatch(s2.l, s2.ana, protocol.TypeLeaveRoom, nil)
	s2.bia.waitFor(t, protocol.MsgInviteExpired, expiredFor(inv2.InviteID))
}

func TestInviteSurvivesInviterLeaving(t *testing.T) {
	s := newScene(t)
	s.joinByID(s.bia)
	waitSeats(t, s.edu, s.roomID, 2)
	s.invite(s.ana, "edu", nil)
	inv := received(t, s.edu)
	dispatch(s.l, s.ana, protocol.TypeLeaveRoom, nil)
	s.ana.waitFor(t, protocol.MsgLeftRoom, nil)
	quiet()
	if s.edu.count(protocol.MsgInviteExpired) != 0 {
		t.Fatal("convite é da sala: não expira quando quem convidou sai")
	}
	s.respond(s.edu, inv.InviteID, true)
	if st := roomState(t, s.edu); st.ID != s.roomID {
		t.Fatalf("room_state = %+v", st)
	}
}

func TestTwoAcceptsForLastSeat(t *testing.T) {
	s := newScene(t)
	s.joinByID(s.bia)
	s.joinByID(s.caio)
	waitSeats(t, s.edu, s.roomID, 3)
	s.invite(s.ana, "duda", nil)
	s.invite(s.ana, "edu", nil)
	invDuda, invEdu := received(t, s.duda), received(t, s.edu)
	accept := func(c *fakeClient, id string) {
		msg, _ := json.Marshal(protocol.InviteRespondPayload{InviteID: id, Accept: true})
		s.l.Dispatch(c, protocol.ClientMessage{Type: protocol.TypeInviteRespond, Seq: 7, Payload: msg})
	}
	accept(s.duda, invDuda.InviteID)
	accept(s.edu, invEdu.InviteID)

	seated := func(c *fakeClient) bool { return c.count(protocol.MsgRoomState) > 0 }
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !(seated(s.duda) || seated(s.edu)) {
		time.Sleep(time.Millisecond)
	}
	winner, loser, loserInv := s.duda, s.edu, invEdu
	if seated(s.edu) {
		winner, loser, loserInv = s.edu, s.duda, invDuda
	}
	roomState(t, winner)
	loser.waitFor(t, protocol.MsgError, func(m protocol.ServerMessage) bool {
		p := m.Payload.(protocol.ErrorPayload)
		return p.Code == protocol.CodeInviteExpired && p.Seq == 7
	})
	loser.waitFor(t, protocol.MsgInviteExpired, expiredFor(loserInv.InviteID))
	if loser.count(protocol.MsgRoomState) != 0 {
		t.Fatal("quem perdeu a corrida não pode ter assento")
	}
	// sem reserva presa: se o Join chegou a ser reservado, o lobby volta; em qualquer caso ela cria a própria sala
	// (o aceite pode ser recusado antes do Join, quando a sala já avisou que encheu)
	dispatch(s.l, loser, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPrivate})
	roomState(t, loser)
}

func TestTokenOpenedByAnotherAccount(t *testing.T) {
	s := newScene(t)
	waitSeats(t, s.edu, s.roomID, 1)
	s.invite(s.ana, "fabi", nil)
	waitInvites(t, s.ana, hasState("fabi", protocol.InvitePending))
	token := s.people.mails()[0].token

	dispatch(s.l, s.bia, protocol.TypeInviteAcceptToken, protocol.InviteAcceptTokenPayload{Token: token})
	s.bia.waitFor(t, protocol.MsgError, errorCode(protocol.CodeInviteNotYours))
	quiet()
	if s.bia.count(protocol.MsgRoomState) != 0 {
		t.Fatal("outra conta não pode sentar com o token")
	}
	if i, _ := stateOf(waitInvites(t, s.ana, func(l []protocol.RoomInvite) bool { return len(l) == 1 }), "fabi"); i.State != protocol.InvitePending {
		t.Fatalf("o convite deve seguir pendente para o convidado: %+v", i)
	}

	fabi := connect(s.l, "u5", "Fabi")
	dispatch(s.l, fabi, protocol.TypeInviteAcceptToken, protocol.InviteAcceptTokenPayload{Token: token})
	if st := roomState(t, fabi); st.ID != s.roomID {
		t.Fatalf("room_state = %+v", st)
	}
	waitInvites(t, s.ana, hasState("fabi", protocol.InviteAccepted))
	// depois de aceito o token não resolve mais
	dispatch(s.l, s.caio, protocol.TypeInviteAcceptToken, protocol.InviteAcceptTokenPayload{Token: token})
	s.caio.waitFor(t, protocol.MsgError, errorCode(protocol.CodeInviteExpired))
}

func TestUnknownTokenIsExpired(t *testing.T) {
	s := newScene(t)
	dispatch(s.l, s.bia, protocol.TypeInviteAcceptToken, protocol.InviteAcceptTokenPayload{Token: "token-de-antes-do-restart"})
	s.bia.waitFor(t, protocol.MsgError, errorCode(protocol.CodeInviteExpired))
}

func TestRespondWithForeignOrUnknownID(t *testing.T) {
	s := newScene(t)
	waitSeats(t, s.edu, s.roomID, 1)
	s.invite(s.ana, "bia", nil)
	inv := received(t, s.bia)
	s.respond(s.caio, inv.InviteID, true)
	s.caio.waitFor(t, protocol.MsgError, errorCode(protocol.CodeInviteNotFound))
	s.respond(s.caio, "nao-existe", true)
	s.caio.waitFor(t, protocol.MsgError, errorCode(protocol.CodeInviteExpired))
	quiet()
	if s.caio.count(protocol.MsgRoomState) != 0 {
		t.Fatal("id alheio não pode sentar ninguém")
	}
}

func TestDropUserExpiresInvites(t *testing.T) {
	s := newScene(t)
	waitSeats(t, s.edu, s.roomID, 1)
	s.invite(s.ana, "bia", nil)
	received(t, s.bia)
	waitInvites(t, s.ana, hasState("bia", protocol.InvitePending))
	s.people.forget("bia")
	s.l.DropUser("u1")
	waitInvites(t, s.ana, func(l []protocol.RoomInvite) bool { return len(l) == 0 })
	s.invite(s.ana, "bia", nil)
	s.ana.waitFor(t, protocol.MsgError, errorCode(protocol.CodeUserNotFound))
}

func TestDropUserOfflineInviteeExpiresInvite(t *testing.T) {
	s := newScene(t)
	waitSeats(t, s.edu, s.roomID, 1)
	s.invite(s.ana, "fabi", nil)
	waitInvites(t, s.ana, hasState("fabi", protocol.InvitePending))
	s.l.DropUser("u5") // desativado sem estar conectado
	waitInvites(t, s.ana, func(l []protocol.RoomInvite) bool { return len(l) == 0 })
}

func TestDropUserDuringLookupBlocksInvite(t *testing.T) {
	s := newScene(t)
	waitSeats(t, s.edu, s.roomID, 1)
	hold := make(chan struct{})
	s.people.holds["bia"] = hold
	s.invite2go(s.ana, "bia")
	select {
	case <-s.people.asked:
	case <-time.After(2 * time.Second):
		t.Fatal("a consulta não começou")
	}
	s.l.DropUser("u1") // desativada entre a consulta (fora do mutex) e a criação do convite
	close(hold)
	s.ana.waitFor(t, protocol.MsgError, errorCode(protocol.CodeUserNotFound))
	quiet()
	if s.bia.count(protocol.MsgInviteReceived) != 0 {
		t.Fatal("convite criado para usuário derrubado")
	}
}

func (s *scene) invite2go(c *fakeClient, username string) {
	go s.invite(c, username, nil)
}

func TestReconnectResendsPendingInvites(t *testing.T) {
	s := newScene(t)
	waitSeats(t, s.edu, s.roomID, 1)
	s.invite(s.ana, "bia", nil)
	first := received(t, s.bia)
	bia2 := connect(s.l, "u1", "Bia") // aba recarregada: nova conexão substitui a antiga
	again := received(t, bia2)
	if again.InviteID != first.InviteID || again.Room.ID != s.roomID {
		t.Fatalf("reenvio = %+v, queria o mesmo convite %s", again, first.InviteID)
	}
	s.respond(bia2, again.InviteID, true)
	if st := roomState(t, bia2); st.ID != s.roomID {
		t.Fatalf("room_state = %+v", st)
	}
	// quem já está em sala recebe de volta a lista de convites da sala ao reconectar
	ana2 := connect(s.l, "u0", "Ana")
	waitInvites(t, ana2, hasState("bia", protocol.InviteAccepted))
}

func TestJoiningByCodeAcceptsPendingInvite(t *testing.T) {
	s := newScene(t)
	waitSeats(t, s.edu, s.roomID, 1)
	s.invite(s.ana, "bia", nil)
	inv := received(t, s.bia)
	s.joinByID(s.bia)
	roomState(t, s.bia)
	s.bia.waitFor(t, protocol.MsgInviteExpired, expiredFor(inv.InviteID))
	waitInvites(t, s.ana, hasState("bia", protocol.InviteAccepted))
}

func TestFullRoomJoinDoesNotFlickerPresence(t *testing.T) {
	s := newScene(t)
	s.joinByID(s.bia)
	s.joinByID(s.caio)
	s.joinByID(s.duda)
	waitSeats(t, s.edu, s.roomID, 4)
	waitStatus(t, s.ana, "u3", protocol.PresenceWaiting)
	s.joinByID(s.edu)
	s.edu.waitFor(t, protocol.MsgError, errorCode(protocol.CodeRoomFull))
	quiet()
	for _, u := range presenceUpdates(s.ana, "u4") {
		if u.Status != protocol.PresenceLobby {
			t.Fatalf("tentar sala cheia não pode mexer na presença: %+v", u)
		}
	}
}

func TestReconnectDoesNotRebroadcastSamePresence(t *testing.T) {
	s := newScene(t)
	waitStatus(t, s.ana, "u1", protocol.PresenceLobby)
	before := len(presenceUpdates(s.ana, "u1"))
	connect(s.l, "u1", "Bia")
	quiet()
	if after := len(presenceUpdates(s.ana, "u1")); after != before {
		t.Fatalf("reconexão com a mesma presença retransmitiu: %d -> %d", before, after)
	}
}

func respondSeq(l *lobby.Lobby, c *fakeClient, id string, accept bool, seq int64) {
	raw, _ := json.Marshal(protocol.InviteRespondPayload{InviteID: id, Accept: accept})
	l.Dispatch(c, protocol.ClientMessage{Type: protocol.TypeInviteRespond, Seq: seq, Payload: raw})
}

func everSaw(c *fakeClient, username string, st protocol.InviteState) bool {
	for _, m := range c.messages() {
		if m.Type != protocol.MsgRoomInvites {
			continue
		}
		if i, ok := stateOf(m.Payload.(protocol.RoomInvitesPayload).Invites, username); ok && i.State == st {
			return true
		}
	}
	return false
}

// Aceite com a entrada por código em voo: "aceitou" só aparece se a pessoa realmente sentou; senão o aceite
// termina em erro com o seq dele (nunca fica sem resposta).
func TestAcceptWithJoinInFlightNeverClaimsAcceptedWithoutSeat(t *testing.T) {
	for i := 0; i < 30; i++ {
		s := newScene(t)
		s.joinByID(s.caio)
		s.joinByID(s.duda)
		waitSeats(t, s.edu, s.roomID, 3)
		s.invite(s.ana, "bia", nil)
		inv := received(t, s.bia)
		s.joinByID(s.bia) // entrada por código em voo...
		respondSeq(s.l, s.bia, inv.InviteID, true, 9)
		s.joinByID(s.edu) // ...disputando a última vaga
		time.Sleep(40 * time.Millisecond)
		seated := s.bia.count(protocol.MsgRoomState) > 0
		if seated {
			waitInvites(t, s.ana, hasState("bia", protocol.InviteAccepted))
			continue
		}
		if everSaw(s.ana, "bia", protocol.InviteAccepted) {
			t.Fatalf("iteração %d: lista mostrou aceitou sem a pessoa sentar", i)
		}
		s.bia.waitFor(t, protocol.MsgError, func(m protocol.ServerMessage) bool {
			p := m.Payload.(protocol.ErrorPayload)
			return p.Seq == 9 && p.Code == protocol.CodeInviteExpired
		})
	}
}

func TestDeclineDuringAcceptIsIgnored(t *testing.T) {
	s := newScene(t)
	dispatch(s.l, s.bia, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPrivate})
	roomState(t, s.bia)
	waitSeats(t, s.edu, s.roomID, 1)
	s.invite(s.ana, "bia", nil)
	inv := received(t, s.bia)
	s.respond(s.bia, inv.InviteID, true)
	s.respond(s.bia, inv.InviteID, false) // aceite em andamento: a recusa não vale
	waitInvites(t, s.ana, hasState("bia", protocol.InviteAccepted))
	if everSaw(s.ana, "bia", protocol.InviteDeclined) {
		t.Fatal("recusa durante o aceite não pode marcar recusou")
	}
}

// Desconectar no meio da saída da sala atual não pode gastar o convite: ou a pessoa já sentou na sala do convite
// ou o convite segue pendente e o Connect reenvia o aviso.
func TestDisconnectDuringMoveKeepsInvite(t *testing.T) {
	for i := 0; i < 30; i++ {
		s := newScene(t)
		dispatch(s.l, s.bia, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPrivate})
		roomState(t, s.bia)
		waitSeats(t, s.edu, s.roomID, 1)
		s.invite(s.ana, "bia", nil)
		inv := received(t, s.bia)
		s.respond(s.bia, inv.InviteID, true)
		s.l.Disconnect(s.bia)
		time.Sleep(40 * time.Millisecond)
		list := waitInvites(t, s.ana, func(l []protocol.RoomInvite) bool { _, ok := stateOf(l, "bia"); return ok })
		last := s.ana.messages()
		for k := len(last) - 1; k >= 0; k-- {
			if last[k].Type == protocol.MsgRoomInvites {
				list = last[k].Payload.(protocol.RoomInvitesPayload).Invites
				break
			}
		}
		st, ok := stateOf(list, "bia")
		if !ok {
			t.Fatalf("iteração %d: convite sumiu da lista: %+v", i, list)
		}
		if st.State == protocol.InvitePending {
			again := connect(s.l, "u1", "Bia")
			if got := received(t, again); got.InviteID != inv.InviteID {
				t.Fatalf("iteração %d: reenvio = %+v", i, got)
			}
		}
	}
}
