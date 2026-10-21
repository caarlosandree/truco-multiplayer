package lobby

import (
	"slices"
	"strings"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

// entryLocked deriva a presença do usuário de clients + index + resumo da sala; ok=false se não está conectado.
func (l *Lobby) entryLocked(userID string) (protocol.PresenceEntry, bool) {
	m := l.clients[userID]
	if m == nil {
		return protocol.PresenceEntry{}, false
	}
	e := protocol.PresenceEntry{UserID: userID, Username: m.info.Username, DisplayName: m.info.DisplayName,
		Status: protocol.PresenceLobby}
	if r := l.roomOfLocked(userID); r != nil {
		e.Status = protocol.PresenceWaiting
		if r.summary.Room.Status == protocol.RoomPlaying {
			e.Status = protocol.PresencePlaying
		}
		if r.summary.Public {
			e.RoomName = r.summary.Room.Name
		} else {
			e.PrivateRoom = true
		}
	}
	return e, true
}

// touchPresenceLocked transmite presence_updated a todos os conectados, só para quem mudou desde o último envio.
func (l *Lobby) touchPresenceLocked(userIDs ...string) {
	if l.closing {
		return
	}
	for _, id := range userIDs {
		e, ok := l.entryLocked(id)
		if !ok || l.lastPresence[id] == e {
			continue
		}
		l.lastPresence[id] = e
		l.broadcastAllLocked(protocol.PresenceUpdated(e), "")
	}
}

// presenceSnapshotLocked lista todos os conectados, ordenados por usuário.
func (l *Lobby) presenceSnapshotLocked() protocol.ServerMessage {
	users := make([]protocol.PresenceEntry, 0, len(l.clients))
	for id := range l.clients {
		if e, ok := l.entryLocked(id); ok {
			users = append(users, e)
		}
	}
	slices.SortFunc(users, func(a, b protocol.PresenceEntry) int { return strings.Compare(a.UserID, b.UserID) })
	return protocol.PresenceSnapshot(users)
}

// membersLocked devolve os usuários cujo índice aponta para a sala.
func (l *Lobby) membersLocked(roomID string) []string {
	var ids []string
	for user, id := range l.index {
		if id == roomID {
			ids = append(ids, user)
		}
	}
	return ids
}

// broadcastAllLocked envia a todos os conectados, exceto exceptID.
func (l *Lobby) broadcastAllLocked(msg protocol.ServerMessage, exceptID string) {
	for id, m := range l.clients {
		if id != exceptID {
			m.client.Send(msg)
		}
	}
}

// forgetLocked tira o usuário da presença e avisa os demais.
func (l *Lobby) forgetLocked(userID string) {
	delete(l.lastPresence, userID)
	l.broadcastAllLocked(protocol.PresenceRemoved(userID), userID)
}

// DropUser implementa admin.SessionDropper: derruba a conexão do usuário desativado na hora.
// O Disconnect que o cliente fizer depois é ignorado, pois a conexão já saiu de clients.
func (l *Lobby) DropUser(userID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.dropSeq++
	if l.lookups > 0 { // uma consulta de convite em andamento pode ter lido a conta ainda ativa
		l.revoked[userID] = l.dropSeq
	}
	l.dropInvitesOfLocked(userID)
	m := l.clients[userID]
	if m == nil {
		return
	}
	delete(l.clients, userID)
	if e := l.roomOfLocked(userID); e != nil {
		e.actor.Push(room.Disconnected{UserID: userID})
	}
	l.forgetLocked(userID)
	m.client.Close(CloseRevoked)
	l.opt.Log.Info("conexão derrubada", "user_id", userID)
}
