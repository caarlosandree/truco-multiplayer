package lobby_test

import (
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/lobby"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
)

// presenceUpdates devolve, em ordem, os presence_updated de um usuário recebidos por c.
func presenceUpdates(c *fakeClient, userID string) []protocol.PresenceEntry {
	var out []protocol.PresenceEntry
	for _, m := range c.messages() {
		if m.Type == protocol.MsgPresenceUpdated {
			if u := m.Payload.(protocol.PresenceUpdatedPayload).User; u.UserID == userID {
				out = append(out, u)
			}
		}
	}
	return out
}

func waitStatus(t *testing.T, c *fakeClient, userID string, status protocol.PresenceStatus) protocol.PresenceEntry {
	t.Helper()
	m := c.waitFor(t, protocol.MsgPresenceUpdated, func(m protocol.ServerMessage) bool {
		u := m.Payload.(protocol.PresenceUpdatedPayload).User
		return u.UserID == userID && u.Status == status
	})
	return m.Payload.(protocol.PresenceUpdatedPayload).User
}

func TestPresenceSnapshotOnConnect(t *testing.T) {
	l := newLobby(t, 3)
	ana := connect(l, "u0", "Ana")
	dispatch(l, ana, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic})
	roomState(t, ana)
	bia := connect(l, "u1", "Bia")
	snap := bia.waitFor(t, protocol.MsgPresenceSnapshot, nil).Payload.(protocol.PresenceSnapshotPayload)
	if len(snap.Users) != 2 || snap.Users[0].UserID != "u0" || snap.Users[0].Status != protocol.PresenceWaiting ||
		snap.Users[0].RoomName != "Sala de Ana" || snap.Users[1].UserID != "u1" || snap.Users[1].Status != protocol.PresenceLobby ||
		snap.Users[1].Username != "bia" {
		t.Fatalf("snapshot = %+v", snap.Users)
	}
	up := waitStatus(t, ana, "u1", protocol.PresenceLobby)
	if up.DisplayName != "Bia" {
		t.Fatalf("updated = %+v", up)
	}
	if n := bia.count(protocol.MsgPresenceUpdated); n != 0 {
		t.Fatalf("quem conecta não recebe updated do próprio: %d", n)
	}
}

func TestPresenceTransitions(t *testing.T) {
	l := newLobby(t, 3)
	ana, bia, caio := connect(l, "u0", "Ana"), connect(l, "u1", "Bia"), connect(l, "u2", "Caio")
	dispatch(l, ana, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic})
	pub := waitStatus(t, caio, "u0", protocol.PresenceWaiting)
	if pub.RoomName != "Sala de Ana" || pub.PrivateRoom {
		t.Fatalf("pública = %+v", pub)
	}
	dispatch(l, bia, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPrivate})
	priv := waitStatus(t, caio, "u1", protocol.PresenceWaiting)
	if priv.RoomName != "" || !priv.PrivateRoom {
		t.Fatalf("privada = %+v", priv)
	}
	dispatch(l, bia, protocol.TypeLeaveRoom, nil)
	waitStatus(t, caio, "u1", protocol.PresenceLobby)
	// o primeiro lobby de Bia vem do Connect; depois waiting e lobby, sem repetição
	if ups := presenceUpdates(caio, "u1"); len(ups) != 2 || ups[0].Status != protocol.PresenceWaiting ||
		ups[1].Status != protocol.PresenceLobby {
		t.Fatalf("updates de Bia = %+v", ups)
	}
	if ups := presenceUpdates(caio, "u0"); len(ups) != 1 {
		t.Fatalf("updates de Ana = %+v", ups)
	}
	l.Disconnect(bia)
	caio.waitFor(t, protocol.MsgPresenceRemoved, func(m protocol.ServerMessage) bool {
		return m.Payload.(protocol.PresenceRemovedPayload).UserID == "u1"
	})
}

func TestPresencePlaying(t *testing.T) {
	l := newLobby(t, 3)
	users := []*fakeClient{connect(l, "u0", "Ana"), connect(l, "u1", "Bia"), connect(l, "u2", "Caio"), connect(l, "u3", "Duda")}
	watcher := connect(l, "u4", "Edu")
	dispatch(l, users[0], protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic})
	id := roomState(t, users[0]).ID
	for _, u := range users[1:] {
		dispatch(l, u, protocol.TypeJoinRoom, protocol.JoinRoomPayload{RoomID: &id})
	}
	for _, u := range users {
		dispatch(l, u, protocol.TypeSetReady, protocol.SetReadyPayload{Ready: true})
	}
	for i, u := range []string{"u0", "u1", "u2", "u3"} {
		e := waitStatus(t, watcher, u, protocol.PresencePlaying)
		if e.RoomName != "Sala de Ana" {
			t.Fatalf("%d: %+v", i, e)
		}
	}
	// quem tenta entrar na sala em jogo é recusado e não aparece "jogando"
	dispatch(l, watcher, protocol.TypeJoinRoom, protocol.JoinRoomPayload{RoomID: &id})
	watcher.waitFor(t, protocol.MsgError, nil)
	if ups := presenceUpdates(users[0], "u4"); len(ups) != 1 || ups[0].Status != protocol.PresenceLobby {
		t.Fatalf("updates de Edu = %+v", ups)
	}
}

func TestDropUserLeavesPresence(t *testing.T) {
	l := newLobby(t, 3)
	ana, bia := connect(l, "u0", "Ana"), connect(l, "u1", "Bia")
	dispatch(l, ana, protocol.TypeCreateRoom, protocol.CreateRoomPayload{Visibility: protocol.VisibilityPublic})
	id := roomState(t, ana).ID
	dispatch(l, bia, protocol.TypeJoinRoom, protocol.JoinRoomPayload{RoomID: &id})
	roomState(t, bia)
	l.DropUser("u1")
	if bia.closeReason() != lobby.CloseRevoked {
		t.Fatalf("fechamento = %v", bia.closeReason())
	}
	ana.waitFor(t, protocol.MsgPresenceRemoved, func(m protocol.ServerMessage) bool {
		return m.Payload.(protocol.PresenceRemovedPayload).UserID == "u1"
	})
	ana.waitFor(t, protocol.MsgRoomState, func(m protocol.ServerMessage) bool {
		st := m.Payload.(protocol.RoomStatePayload)
		return st.Seats[1] != nil && !st.Seats[1].Connected
	})
	before, snaps := ana.count(protocol.MsgPresenceRemoved), ana.count(protocol.MsgLobbySnapshot)
	l.Disconnect(bia) // o laço de escrita do cliente ainda chama Disconnect: tem de ser ignorado
	dispatch(l, bia, protocol.TypeLeaveRoom, nil)
	l.DropUser("u1") // idempotente
	if ana.count(protocol.MsgPresenceRemoved) != before {
		t.Fatal("Disconnect posterior não pode repetir presence_removed")
	}
	if ana.count(protocol.MsgLobbySnapshot) != snaps {
		t.Fatal("Ana não deveria ter saído da sala")
	}
}
