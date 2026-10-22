package lobby

import (
	"cmp"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"
	"slices"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/pagination"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

const (
	inviteLookupTimeout = 5 * time.Second
	maxUsernameLen      = 64
	maxTokenLen         = 128
)

// invite é um convite em memória (some no restart, como as salas de espera). Convites expirados são removidos;
// accepted e declined ficam até a sala fechar ou um novo convite substituir a linha.
type invite struct {
	id, roomID, token string
	inviterName       string
	invitee           Invitee
	seat              *engine.Seat
	state             protocol.InviteState
	emailed           bool
	n                 uint64 // ordem de criação
}

// inviteMove é o aceite de quem ainda está sentado noutra sala de espera: espera o Released dela.
type inviteMove struct {
	inviteID, fromRoom string
	seq                int64
}

// acceptance é a entrada em andamento de quem aceitou um convite. ownJoin=false quando o Join não é nosso (o usuário
// já entrava pelo código): nesse caso o aceite não tem erro próprio da sala e, se a entrada falhar, o lobby responde
// invite_expired com o seq do aceite.
type acceptance struct {
	inviteID string
	seq      int64
	ownJoin  bool
}

// inviteBook guarda o estado dos convites; só é tocado com Lobby.mu travado.
type inviteBook struct {
	invites   map[string]*invite
	byRoom    map[string]map[string]*invite // sala → convites
	byInvitee map[string]map[string]*invite // convidado → convites
	tokens    map[string]string             // token → convite pendente
	mailed    map[string]map[string]bool    // sala → convidado → já recebeu e-mail
	mailsBy   *mailQuota                    // e-mails de convite por quem convida (inviteMailsPerInviter/h)
	mailsTo   *mailQuota                    // e-mails de convite por convidado (inviteMailsPerInvitee/dia)
	moves     map[string]inviteMove         // convidado → aceite à espera de sair da sala atual
	accepting map[string]acceptance         // convidado → entrada (Join) em andamento
	seated    map[string]string             // usuário → sala em que o assento foi confirmado (Seated)
	revoked   map[string]uint64             // derrubado → DropUser que o derrubou; só vive enquanto há consulta em voo
	lookups   int                           // consultas a People.Invitee em andamento (fora do mutex)
	dropSeq   uint64
	inviteSeq uint64
}

func newInviteBook() inviteBook {
	return inviteBook{invites: map[string]*invite{}, byRoom: map[string]map[string]*invite{},
		byInvitee: map[string]map[string]*invite{}, tokens: map[string]string{}, mailed: map[string]map[string]bool{},
		mailsBy: newMailQuota(inviteMailsPerInviter, time.Hour), mailsTo: newMailQuota(inviteMailsPerInvitee, 24*time.Hour),
		moves: map[string]inviteMove{}, accepting: map[string]acceptance{}, seated: map[string]string{}, revoked: map[string]uint64{}}
}

// randomString sorteia n bytes do gerador do lobby (crypto/rand em produção) em base64url sem padding.
func (l *Lobby) randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(l.rand, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func freeSeats(s room.Summary) int {
	n := 0
	for _, st := range s.Room.Seats {
		if st == nil || st.Bot {
			n++
		}
	}
	return n
}

// ---- índices --------------------------------------------------------------------------------------------------

func (l *Lobby) addInviteLocked(inv *invite) {
	l.invites[inv.id] = inv
	l.tokens[inv.token] = inv.id
	if l.byRoom[inv.roomID] == nil {
		l.byRoom[inv.roomID] = map[string]*invite{}
	}
	l.byRoom[inv.roomID][inv.id] = inv
	if l.byInvitee[inv.invitee.ID] == nil {
		l.byInvitee[inv.invitee.ID] = map[string]*invite{}
	}
	l.byInvitee[inv.invitee.ID][inv.id] = inv
}

func (l *Lobby) removeInviteLocked(inv *invite) {
	delete(l.invites, inv.id)
	delete(l.tokens, inv.token)
	delete(l.byRoom[inv.roomID], inv.id)
	if len(l.byRoom[inv.roomID]) == 0 {
		delete(l.byRoom, inv.roomID)
	}
	delete(l.byInvitee[inv.invitee.ID], inv.id)
	if len(l.byInvitee[inv.invitee.ID]) == 0 {
		delete(l.byInvitee, inv.invitee.ID)
	}
}

// setStateLocked muda o estado; fora de pending o token deixa de resolver.
func (l *Lobby) setStateLocked(inv *invite, st protocol.InviteState) {
	inv.state = st
	if st != protocol.InvitePending {
		delete(l.tokens, inv.token)
	}
}

func (l *Lobby) pendingInviteLocked(roomID, userID string) *invite {
	for _, inv := range l.byInvitee[userID] {
		if inv.roomID == roomID && inv.state == protocol.InvitePending {
			return inv
		}
	}
	return nil
}

// pendingOfLocked lista os pendentes do convidado, do mais antigo ao mais novo.
func (l *Lobby) pendingOfLocked(userID string) []*invite {
	var out []*invite
	for _, inv := range l.byInvitee[userID] {
		if inv.state == protocol.InvitePending {
			out = append(out, inv)
		}
	}
	slices.SortFunc(out, func(a, b *invite) int { return cmp.Compare(a.n, b.n) })
	return out
}

// ---- mensagens -----------------------------------------------------------------------------------------------

func (l *Lobby) receivedMsgLocked(inv *invite, e *entry) protocol.ServerMessage {
	vis := protocol.VisibilityPrivate
	if e.summary.Public {
		vis = protocol.VisibilityPublic
	}
	var seat *engine.Seat
	if inv.seat != nil {
		s := *inv.seat
		seat = &s
	}
	return protocol.InviteReceived(protocol.InviteReceivedPayload{InviteID: inv.id,
		Room: protocol.InviteRoom{ID: e.summary.Room.ID, Name: e.summary.Room.Name, Visibility: vis},
		From: protocol.InviteFrom{DisplayName: inv.inviterName}, Seat: seat, FreeSeats: freeSeats(e.summary)})
}

func (l *Lobby) roomInvitesMsgLocked(roomID string) protocol.ServerMessage {
	list := make([]*invite, 0, len(l.byRoom[roomID]))
	for _, inv := range l.byRoom[roomID] {
		list = append(list, inv)
	}
	slices.SortFunc(list, func(a, b *invite) int { return cmp.Compare(a.n, b.n) })
	out := make([]protocol.RoomInvite, 0, len(list))
	for _, inv := range list {
		var seat *engine.Seat
		if inv.seat != nil {
			s := *inv.seat
			seat = &s
		}
		out = append(out, protocol.RoomInvite{InviteID: inv.id, Username: inv.invitee.Username,
			DisplayName: inv.invitee.DisplayName, State: inv.state, Emailed: inv.emailed, Seat: seat})
	}
	return protocol.RoomInvites(out)
}

func (l *Lobby) broadcastRoomInvitesLocked(roomID string) {
	msg := l.roomInvitesMsgLocked(roomID)
	for _, user := range l.membersLocked(roomID) {
		if m := l.clients[user]; m != nil {
			m.client.Send(msg)
		}
	}
}

func (l *Lobby) sendLocked(userID string, msg protocol.ServerMessage) {
	if m := l.clients[userID]; m != nil {
		m.client.Send(msg)
	}
}

// expireLocked remove o convite e retira o aviso do convidado; quem chama decide se avisa a sala.
func (l *Lobby) expireLocked(inv *invite) {
	l.removeInviteLocked(inv)
	l.sendLocked(inv.invitee.ID, protocol.InviteExpired(inv.id))
}

// expireRoomPendingLocked expira os pendentes da sala (encheu ou a partida começou) e atualiza a lista dela.
func (l *Lobby) expireRoomPendingLocked(roomID string) {
	changed := false
	for _, inv := range slices.Collect(mapValues(l.byRoom[roomID])) {
		if inv.state == protocol.InvitePending {
			l.expireLocked(inv)
			changed = true
		}
	}
	if changed {
		l.broadcastRoomInvitesLocked(roomID)
	}
}

// dropRoomInvitesLocked apaga todos os convites da sala que fechou, avisando os pendentes.
func (l *Lobby) dropRoomInvitesLocked(roomID string) {
	for _, inv := range slices.Collect(mapValues(l.byRoom[roomID])) {
		if inv.state == protocol.InvitePending {
			l.expireLocked(inv)
		} else {
			l.removeInviteLocked(inv)
		}
	}
	delete(l.mailed, roomID)
}

func mapValues[K comparable, V any](m map[K]V) func(yield func(V) bool) {
	return func(yield func(V) bool) {
		for _, v := range m {
			if !yield(v) {
				return
			}
		}
	}
}

// ---- convidar ------------------------------------------------------------------------------------------------

// dispatchInvite trata `invite` fora do mutex durante a consulta ao cadastro (ela pode ir ao banco); depois
// revalida tudo sob o mutex. Roda na goroutine de leitura do remetente, então só atrasa as mensagens dele.
func (l *Lobby) dispatchInvite(c Client, msg protocol.ClientMessage) {
	id := c.UserID()
	l.mu.Lock()
	m := l.clients[id]
	if l.closing || m == nil || m.client != c {
		l.mu.Unlock()
		return
	}
	var p protocol.InvitePayload
	if err := protocol.Decode(msg.Payload, &p, "username"); err != nil || p.Username == "" || len(p.Username) > maxUsernameLen {
		l.mu.Unlock()
		c.Send(protocol.ErrorMsg(protocol.CodeBadRequest, msg.Seq))
		return
	}
	if !pagination.ValidText(p.Username) { // nunca vai ao banco: é só alguém que não existe, sem log de erro
		l.mu.Unlock()
		c.Send(protocol.ErrorMsg(protocol.CodeUserNotFound, msg.Seq))
		return
	}
	if _, in := l.index[id]; !in { // evita a consulta de quem nem está em sala
		l.mu.Unlock()
		c.Send(protocol.ErrorMsg(protocol.CodeNotInRoom, msg.Seq))
		return
	}
	startedAt := l.dropSeq
	l.lookups++
	l.mu.Unlock()

	ctx, cancel := context.WithTimeout(l.ctx, inviteLookupTimeout)
	who, lookupErr := l.opt.People.Invitee(ctx, p.Username)
	cancel()

	l.mu.Lock()
	defer l.mu.Unlock()
	// derrubado (conta desativada) depois que a consulta começou: o resultado dela pode estar velho
	if lookupErr == nil && l.revoked[who.ID] > startedAt {
		lookupErr = ErrNoInvitee
	}
	if l.lookups--; l.lookups == 0 {
		clear(l.revoked) // sem consulta em voo, as marcas não servem mais: o conjunto não cresce sem limite
	}
	if m = l.clients[id]; l.closing || m == nil || m.client != c {
		return
	}
	l.inviteLocked(m, p, who, lookupErr, msg.Seq)
}

func (l *Lobby) inviteLocked(m *member, p protocol.InvitePayload, who Invitee, lookupErr error, seq int64) {
	fail := func(code string) { m.client.Send(protocol.ErrorMsg(code, seq)) }
	e := l.roomOfLocked(m.info.ID)
	switch {
	case e == nil:
		fail(protocol.CodeNotInRoom)
		return
	case e.summary.Room.Status != protocol.RoomWaiting:
		fail(protocol.CodeMatchInProgress)
		return
	case freeSeats(e.summary) == 0:
		fail(protocol.CodeRoomFull)
		return
	}
	if p.Seat != nil {
		if *p.Seat > 3 {
			fail(protocol.CodeBadRequest)
			return
		}
		if e.summary.Room.Seats[*p.Seat] != nil {
			fail(protocol.CodeSeatTaken)
			return
		}
	}
	if lookupErr != nil {
		if errors.Is(lookupErr, ErrNoInvitee) {
			fail(protocol.CodeUserNotFound)
			return
		}
		l.opt.Log.Error("falha ao consultar convidado", "user_id", m.info.ID, "err", lookupErr)
		fail(protocol.CodeInternal)
		return
	}
	roomID := e.summary.Room.ID
	switch {
	case who.ID == m.info.ID:
		fail(protocol.CodeCannotInviteSelf)
		return
	case l.index[who.ID] == roomID:
		fail(protocol.CodeAlreadyInRoom)
		return
	case l.pendingInviteLocked(roomID, who.ID) != nil:
		m.client.Send(l.roomInvitesMsgLocked(roomID)) // já convidado: sem novo aviso nem e-mail
		return
	}
	id, err := l.randomString(16)
	var token string
	if err == nil {
		token, err = l.randomString(32)
	}
	if err != nil {
		l.opt.Log.Error("falha ao gerar convite", "err", err)
		fail(protocol.CodeInternal)
		return
	}
	for _, old := range slices.Collect(mapValues(l.byInvitee[who.ID])) { // recusado/aceito antigo dá lugar ao novo
		if old.roomID == roomID {
			l.removeInviteLocked(old)
		}
	}
	l.inviteSeq++
	inv := &invite{id: id, roomID: roomID, token: token, inviterName: m.info.DisplayName, invitee: who,
		state: protocol.InvitePending, n: l.inviteSeq}
	if p.Seat != nil {
		s := *p.Seat
		inv.seat = &s
	}
	l.addInviteLocked(inv)
	if online := l.clients[who.ID]; online != nil {
		online.client.Send(l.receivedMsgLocked(inv, e))
	} else if !l.mailed[roomID][who.ID] {
		if now := l.opt.Clock.Now(); l.mailsBy.allows(m.info.ID, now) && l.mailsTo.allows(who.ID, now) {
			if l.mailed[roomID] == nil {
				l.mailed[roomID] = map[string]bool{}
			}
			l.mailed[roomID][who.ID] = true
			l.mailsBy.record(m.info.ID, now)
			l.mailsTo.record(who.ID, now)
			inv.emailed = true
			l.opt.People.MailInvite(who.ID, m.info.DisplayName, e.summary.Room.Name, token) // não bloqueia
		} else {
			l.opt.Log.Info("teto de e-mail de convite atingido; convite só no app", "inviter_id", m.info.ID, "invitee_id", who.ID)
		}
	}
	l.broadcastRoomInvitesLocked(roomID)
	l.opt.Log.Info("convite criado", "invite_id", id, "room_id", roomID, "inviter_id", m.info.ID, "invitee_id", who.ID,
		"emailed", inv.emailed)
}

// ---- responder / aceitar -------------------------------------------------------------------------------------

func (l *Lobby) inviteRespondLocked(m *member, msg protocol.ClientMessage) {
	var p protocol.InviteRespondPayload
	if err := protocol.Decode(msg.Payload, &p, "inviteId", "accept"); err != nil || len(p.InviteID) > maxTokenLen {
		m.client.Send(protocol.ErrorMsg(protocol.CodeBadRequest, msg.Seq))
		return
	}
	inv := l.invites[p.InviteID]
	switch {
	case inv == nil:
		m.client.Send(protocol.ErrorMsg(protocol.CodeInviteExpired, msg.Seq))
		return
	case inv.invitee.ID != m.info.ID:
		m.client.Send(protocol.ErrorMsg(protocol.CodeInviteNotFound, msg.Seq))
		return
	}
	if !p.Accept {
		if l.acceptingInviteLocked(m.info.ID, inv.id) {
			return // há aceite deste convite em andamento: recusar agora deixaria "recusou" e a pessoa sentada
		}
		if inv.state == protocol.InvitePending {
			l.setStateLocked(inv, protocol.InviteDeclined)
			l.broadcastRoomInvitesLocked(inv.roomID)
		}
		m.client.Send(protocol.InviteExpired(inv.id)) // retira o aviso; também é a confirmação
		return
	}
	if inv.state != protocol.InvitePending {
		m.client.Send(protocol.ErrorMsg(protocol.CodeInviteExpired, msg.Seq))
		return
	}
	l.acceptLocked(m, inv, msg.Seq)
}

// acceptingInviteLocked diz se o aceite deste convite pelo usuário ainda está em andamento (entrada ou saída da sala atual).
func (l *Lobby) acceptingInviteLocked(userID, inviteID string) bool {
	if a, ok := l.accepting[userID]; ok && a.inviteID == inviteID {
		return true
	}
	mv, ok := l.moves[userID]
	return ok && mv.inviteID == inviteID
}

func (l *Lobby) inviteTokenLocked(m *member, msg protocol.ClientMessage) {
	var p protocol.InviteAcceptTokenPayload
	if err := protocol.Decode(msg.Payload, &p, "token"); err != nil || p.Token == "" || len(p.Token) > maxTokenLen {
		m.client.Send(protocol.ErrorMsg(protocol.CodeBadRequest, msg.Seq))
		return
	}
	inv := l.invites[l.tokens[p.Token]] // só tokens de convite pendente ficam no mapa
	if inv == nil || inv.state != protocol.InvitePending || subtle.ConstantTimeCompare([]byte(inv.token), []byte(p.Token)) != 1 {
		m.client.Send(protocol.ErrorMsg(protocol.CodeInviteExpired, msg.Seq))
		return
	}
	if inv.invitee.ID != m.info.ID { // e-mail encaminhado: o convite segue pendente para o convidado
		m.client.Send(protocol.ErrorMsg(protocol.CodeInviteNotYours, msg.Seq))
		return
	}
	l.acceptLocked(m, inv, msg.Seq)
}

// acceptLocked aceita um convite pendente do próprio usuário (já conferido por quem chama).
func (l *Lobby) acceptLocked(m *member, inv *invite, seq int64) {
	user := m.info.ID
	if _, busy := l.accepting[user]; busy {
		return // já há um aceite em andamento; este pedido não tem efeito
	}
	if _, busy := l.moves[user]; busy {
		return
	}
	target := l.rooms[inv.roomID]
	if target == nil { // a sala fechou: Destroyed já teria apagado o convite; defesa
		l.failInviteLocked(user, inv, seq, false)
		return
	}
	cur, in := l.index[user]
	switch {
	case in && cur == inv.roomID:
		if l.seated[user] == inv.roomID {
			l.markAcceptedLocked(inv)
			// reenvia o room_state: todo aceite por token termina em algo que o cliente vê
			target.actor.Push(room.Join{UserID: user, DisplayName: m.info.DisplayName, Seq: seq, Invite: true})
		} else {
			// o usuário só reservou a vaga (entrada por código em voo): quem decide é Seated ou Released
			l.accepting[user] = acceptance{inviteID: inv.id, seq: seq}
		}
	case in:
		if from := l.rooms[cur]; from != nil && from.summary.Room.Status != protocol.RoomWaiting {
			m.client.Send(protocol.ErrorMsg(protocol.CodeInviteePlaying, seq)) // o convite segue pendente
			return
		} else if from == nil {
			delete(l.index, user)
			l.enterLocked(m, inv, target, seq)
			return
		}
		l.moves[user] = inviteMove{inviteID: inv.id, fromRoom: cur, seq: seq}
		vacate := room.Vacate{UserID: user, Refused: func() { l.vacateRefused(user, inv.id, seq) }}
		if !l.rooms[cur].actor.Push(vacate) { // a sala de origem já sumiu: trata na hora
			delete(l.index, user)
			delete(l.seated, user)
			l.touchPresenceLocked(user)
			l.resolveMoveLocked(user)
		}
	default:
		l.enterLocked(m, inv, target, seq)
	}
}

// enterLocked reserva o índice e pede o assento na sala do convite; quem decide a corrida por vaga é a sala.
func (l *Lobby) enterLocked(m *member, inv *invite, target *entry, seq int64) {
	user := m.info.ID
	l.index[user] = inv.roomID
	l.accepting[user] = acceptance{inviteID: inv.id, seq: seq, ownJoin: true}
	join := room.Join{UserID: user, DisplayName: m.info.DisplayName, Seat: inv.seat, Seq: seq, Invite: true}
	if !target.actor.Push(join) {
		delete(l.index, user)
		delete(l.accepting, user)
		l.failInviteLocked(user, inv, seq, false)
	}
}

// failInviteLocked encerra o aceite que não deu certo: expira o convite (se ainda existir), responde o erro com
// o seq do pedido e, se o usuário ficou fora de sala, devolve o lobby.
func (l *Lobby) failInviteLocked(userID string, inv *invite, seq int64, snapshot bool) {
	if inv.state == protocol.InvitePending && l.invites[inv.id] == inv {
		l.expireLocked(inv)
		l.broadcastRoomInvitesLocked(inv.roomID)
	}
	m := l.clients[userID]
	if m == nil || l.closing {
		return
	}
	m.client.Send(protocol.ErrorMsg(protocol.CodeInviteExpired, seq))
	if snapshot {
		m.client.Send(l.snapshotLocked())
	}
}

// markAcceptedLocked marca o convite como aceito e retira o aviso do convidado.
func (l *Lobby) markAcceptedLocked(inv *invite) {
	l.setStateLocked(inv, protocol.InviteAccepted)
	l.sendLocked(inv.invitee.ID, protocol.InviteExpired(inv.id))
	l.broadcastRoomInvitesLocked(inv.roomID)
}

// vacateRefused é chamado pelo ator da sala de origem (nunca sob l.mu) quando ela está em partida.
func (l *Lobby) vacateRefused(userID, inviteID string, seq int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if mv, ok := l.moves[userID]; ok && mv.inviteID == inviteID {
		delete(l.moves, userID)
	}
	l.sendLocked(userID, protocol.ErrorMsg(protocol.CodeInviteePlaying, seq))
}

// resolveMoveLocked segue o aceite de quem acabou de sair da sala de origem. Reconfere o convite e a sala
// alvo, porque tudo pode ter mudado enquanto a saída corria.
func (l *Lobby) resolveMoveLocked(userID string) {
	mv, ok := l.moves[userID]
	if !ok {
		return
	}
	delete(l.moves, userID)
	m := l.clients[userID]
	inv := l.invites[mv.inviteID]
	var target *entry
	if inv != nil {
		target = l.rooms[inv.roomID]
	}
	if m == nil && !l.closing {
		return // desconectou no meio da saída: o convite segue pendente e o Connect reenvia o aviso
	}
	if m == nil || l.closing || inv == nil || inv.state != protocol.InvitePending || target == nil ||
		target.summary.Room.Status != protocol.RoomWaiting || freeSeats(target.summary) == 0 {
		switch {
		case inv != nil:
			l.failInviteLocked(userID, inv, mv.seq, true)
			if inv.state != protocol.InvitePending { // já tinha deixado de ser pendente: o aviso ainda precisa sumir
				l.sendLocked(userID, protocol.InviteExpired(inv.id))
			}
		default:
			l.sendLocked(userID, protocol.InviteExpired(mv.inviteID))
			l.sendLocked(userID, protocol.ErrorMsg(protocol.CodeInviteExpired, mv.seq))
			if m != nil && !l.closing {
				m.client.Send(l.snapshotLocked())
			}
		}
		return
	}
	l.enterLocked(m, inv, target, mv.seq)
	if _, still := l.accepting[userID]; !still { // o Push falhou: ficou fora de sala
		if m := l.clients[userID]; m != nil {
			m.client.Send(l.snapshotLocked())
		}
	}
}

// dropInvitesOfLocked apaga o que envolve o usuário desativado: convites para ele, aceites e saídas em andamento.
func (l *Lobby) dropInvitesOfLocked(userID string) {
	rooms := map[string]bool{}
	for _, inv := range slices.Collect(mapValues(l.byInvitee[userID])) {
		l.removeInviteLocked(inv)
		rooms[inv.roomID] = true
	}
	delete(l.moves, userID)
	delete(l.accepting, userID)
	for roomID := range rooms {
		l.broadcastRoomInvitesLocked(roomID)
	}
}

// resendInvitesLocked reenvia os avisos pendentes ao reconectar e a lista da sala a quem está sentado.
func (l *Lobby) resendInvitesLocked(c Client, userID string, in *entry) {
	for _, inv := range l.pendingOfLocked(userID) {
		if e := l.rooms[inv.roomID]; e != nil {
			c.Send(l.receivedMsgLocked(inv, e))
		}
	}
	if in != nil {
		c.Send(l.roomInvitesMsgLocked(in.summary.Room.ID))
	}
}
