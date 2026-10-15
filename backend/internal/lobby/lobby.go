// Package lobby é o registro de salas e conexões: lista ao vivo, códigos, índice usuário→sala,
// restauração após restart, abort pelo administrador e desligamento. Todo o estado fica sob um
// mutex; nada aqui bloqueia, porque Actor.Push e Client.Send/Close nunca esperam.
package lobby

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

type CloseReason int

const (
	CloseReplaced CloseReason = iota + 1 // o mesmo usuário abriu outra conexão
	CloseShutdown                        // o servidor está desligando
)

// Client é a conexão de um usuário. Send e Close nunca bloqueiam.
type Client interface {
	UserID() string
	Send(msg protocol.ServerMessage)
	Close(reason CloseReason)
}

type Options struct {
	Timing   room.Timing
	MaxRooms int
	Store    room.MatchStore
	Clock    clock.Clock
	Rand     io.Reader
	Log      *slog.Logger
}

type member struct {
	client Client
	info   protocol.UserInfo
}

type entry struct {
	actor   *room.Actor
	summary room.Summary
}

type Lobby struct {
	opt    Options
	ctx    context.Context
	cancel context.CancelFunc
	rand   io.Reader

	mu      sync.Mutex
	closing bool
	clients map[string]*member // usuário → conexão ativa
	rooms   map[string]*entry  // sala → ator e resumo
	codes   map[string]string  // código → sala
	index   map[string]string  // usuário → sala (uma sala por usuário)
}

func New(ctx context.Context, o Options) *Lobby {
	if o.Log == nil {
		o.Log = slog.Default()
	}
	ctx, cancel := context.WithCancel(ctx)
	return &Lobby{opt: o, ctx: ctx, cancel: cancel, rand: &lockedReader{r: o.Rand},
		clients: map[string]*member{}, rooms: map[string]*entry{}, codes: map[string]string{}, index: map[string]string{}}
}

// lockedReader serializa o gerador compartilhado pelas salas.
type lockedReader struct {
	mu sync.Mutex
	r  io.Reader
}

func (l *lockedReader) Read(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.r.Read(p)
}

// Connect registra a conexão (substituindo a anterior do mesmo usuário) e manda hello e,
// para quem está fora de sala, a lista; quem está em sala recebe o estado pela própria sala.
func (l *Lobby) Connect(c Client, info protocol.UserInfo) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closing {
		return false
	}
	if old := l.clients[info.ID]; old != nil {
		old.client.Close(CloseReplaced)
	}
	l.clients[info.ID] = &member{client: c, info: info}
	c.Send(protocol.Hello(info))
	if e := l.roomOfLocked(info.ID); e != nil {
		e.actor.Push(room.Connected{UserID: info.ID})
		return true
	}
	c.Send(l.snapshotLocked())
	return true
}

// Disconnect esquece a conexão, se ainda for a ativa do usuário, e avisa a sala.
func (l *Lobby) Disconnect(c Client) {
	l.mu.Lock()
	defer l.mu.Unlock()
	id := c.UserID()
	if m := l.clients[id]; m == nil || m.client != c {
		return
	}
	delete(l.clients, id)
	if e := l.roomOfLocked(id); e != nil {
		e.actor.Push(room.Disconnected{UserID: id})
	}
}

// Dispatch trata create_room/join_room e repassa o resto para a sala do usuário.
func (l *Lobby) Dispatch(c Client, msg protocol.ClientMessage) {
	l.mu.Lock()
	defer l.mu.Unlock()
	id := c.UserID()
	m := l.clients[id]
	if l.closing || m == nil || m.client != c {
		return // conexão substituída ou servidor desligando
	}
	switch msg.Type {
	case protocol.TypeCreateRoom:
		l.createRoomLocked(m, msg)
	case protocol.TypeJoinRoom:
		l.joinRoomLocked(m, msg)
	default:
		if !protocol.KnownClientType(msg.Type) {
			c.Send(protocol.ErrorMsg(protocol.CodeBadRequest, msg.Seq))
			return
		}
		e := l.roomOfLocked(id)
		if e == nil {
			c.Send(protocol.ErrorMsg(protocol.CodeNotInRoom, msg.Seq))
			return
		}
		if !e.actor.Push(room.Command{UserID: id, Message: msg}) {
			c.Send(protocol.ErrorMsg(protocol.CodeRoomNotFound, msg.Seq))
		}
	}
}

func (l *Lobby) createRoomLocked(m *member, msg protocol.ClientMessage) {
	var p protocol.CreateRoomPayload
	if err := protocol.Decode(msg.Payload, &p, "visibility"); err != nil || !p.Visibility.Valid() {
		m.client.Send(protocol.ErrorMsg(protocol.CodeBadRequest, msg.Seq))
		return
	}
	if _, in := l.index[m.info.ID]; in {
		m.client.Send(protocol.ErrorMsg(protocol.CodeAlreadyInRoom, msg.Seq))
		return
	}
	if len(l.rooms) >= l.opt.MaxRooms {
		m.client.Send(protocol.ErrorMsg(protocol.CodeTooManyRooms, msg.Seq))
		return
	}
	id, code, err := l.newIdentityLocked()
	if err != nil {
		l.opt.Log.Error("falha ao gerar identificação da sala", "err", err)
		m.client.Send(protocol.ErrorMsg(protocol.CodeInternal, msg.Seq))
		return
	}
	name := "Sala de " + m.info.DisplayName
	public := p.Visibility == protocol.VisibilityPublic
	actor := room.Start(l.ctx, l.roomOptions(id, code, name, public), nil)
	l.rooms[id] = &entry{actor: actor, summary: room.Summary{
		Room: protocol.RoomSummary{ID: id, Name: name, Status: protocol.RoomWaiting}, Public: public, Code: code}}
	l.codes[code] = id
	l.index[m.info.ID] = id
	l.opt.Log.Info("sala criada", "room_id", id, "public", public, "user_id", m.info.ID)
	actor.Push(room.Join{UserID: m.info.ID, DisplayName: m.info.DisplayName, Seq: msg.Seq})
}

func (l *Lobby) joinRoomLocked(m *member, msg protocol.ClientMessage) {
	var p protocol.JoinRoomPayload
	if err := protocol.Decode(msg.Payload, &p); err != nil || (p.RoomID == nil) == (p.Code == nil) {
		m.client.Send(protocol.ErrorMsg(protocol.CodeBadRequest, msg.Seq))
		return
	}
	if _, in := l.index[m.info.ID]; in {
		m.client.Send(protocol.ErrorMsg(protocol.CodeAlreadyInRoom, msg.Seq))
		return
	}
	var e *entry
	if p.Code != nil {
		e = l.rooms[l.codes[strings.ToUpper(strings.TrimSpace(*p.Code))]]
	} else if candidate := l.rooms[*p.RoomID]; candidate != nil && candidate.summary.Public {
		e = candidate
	}
	if e == nil {
		m.client.Send(protocol.ErrorMsg(protocol.CodeRoomNotFound, msg.Seq))
		return
	}
	roomID := e.summary.Room.ID
	l.index[m.info.ID] = roomID // reserva; a sala confirma ou chama Released
	if !e.actor.Push(room.Join{UserID: m.info.ID, DisplayName: m.info.DisplayName, Seat: p.Seat, Seq: msg.Seq}) {
		delete(l.index, m.info.ID)
		m.client.Send(protocol.ErrorMsg(protocol.CodeRoomNotFound, msg.Seq))
	}
}

// Send implementa room.Outbox.
func (l *Lobby) Send(userID string, msg protocol.ServerMessage) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if m := l.clients[userID]; m != nil {
		m.client.Send(msg)
	}
}

// Update implementa room.Registry: guarda o resumo e avisa quem está no lobby.
func (l *Lobby) Update(s room.Summary) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.rooms[s.Room.ID]
	if e == nil {
		return
	}
	e.summary = s
	if s.Public {
		l.broadcastLobbyLocked(protocol.RoomUpserted(s.Room))
	}
}

// Released implementa room.Registry: o usuário volta ao lobby e recebe a lista.
func (l *Lobby) Released(roomID, userID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index[userID] != roomID {
		return
	}
	delete(l.index, userID)
	if m := l.clients[userID]; m != nil && !l.closing {
		m.client.Send(l.snapshotLocked())
	}
}

// Destroyed implementa room.Registry: remove a sala, o código e a linha da lista.
func (l *Lobby) Destroyed(roomID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.rooms[roomID]
	if e == nil {
		return
	}
	delete(l.rooms, roomID)
	delete(l.codes, e.summary.Code)
	var freed []string
	for user, id := range l.index {
		if id == roomID {
			delete(l.index, user)
			freed = append(freed, user)
		}
	}
	l.opt.Log.Info("sala destruída", "room_id", roomID)
	if !l.closing {
		// quem estava reservado/na sala volta ao lobby; o Released tardio da sala não acharia mais o índice
		snap := l.snapshotLocked()
		for _, user := range freed {
			if m := l.clients[user]; m != nil {
				m.client.Send(snap)
			}
		}
	}
	if e.summary.Public {
		l.broadcastLobbyLocked(protocol.RoomRemoved(roomID))
	}
}

// Restore recria as salas das partidas recuperadas no boot: públicas, código novo, todos desconectados.
func (l *Lobby) Restore(recovered []match.Recovered) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, rc := range recovered {
		id, code, err := l.newIdentityLocked()
		if err != nil {
			return err
		}
		name := "Sala de " + rc.Record.Players[0].DisplayName
		var seats [4]*protocol.SeatSummary
		for i, p := range rc.Record.Players {
			seats[i] = &protocol.SeatSummary{DisplayName: p.DisplayName}
		}
		score := rc.State.Score
		actor := room.Start(l.ctx, l.roomOptions(id, code, name, true),
			&room.Restored{MatchID: rc.Record.ID, Players: rc.Record.Players, State: rc.State, NextPly: rc.NextPly})
		l.rooms[id] = &entry{actor: actor, summary: room.Summary{
			Room:   protocol.RoomSummary{ID: id, Name: name, Status: protocol.RoomPlaying, Seats: seats, Score: &score},
			Public: true, Code: code, MatchID: rc.Record.ID}}
		l.codes[code] = id
		for _, p := range rc.Record.Players {
			if prev, in := l.index[p.UserID]; in {
				l.opt.Log.Warn("usuário em duas partidas recuperadas; mantida a primeira sala", "user_id", p.UserID,
					"match_id", l.rooms[prev].summary.MatchID, "other_match_id", rc.Record.ID)
				continue
			}
			l.index[p.UserID] = id
		}
		l.opt.Log.Info("sala restaurada", "room_id", id, "match_id", rc.Record.ID)
	}
	return nil
}

// Abort implementa admin.MatchAborter: encerra a sala viva da partida, se houver.
func (l *Lobby) Abort(_ context.Context, matchID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.rooms {
		if e.summary.MatchID == matchID {
			e.actor.Push(room.Abort{Reason: "admin", MatchID: matchID})
		}
	}
	return nil
}

// Shutdown recusa novidades, para as salas (as partidas ficam para o Recover) e fecha as conexões.
func (l *Lobby) Shutdown(ctx context.Context) error {
	l.mu.Lock()
	l.closing = true
	actors := make([]*room.Actor, 0, len(l.rooms))
	for _, e := range l.rooms {
		actors = append(actors, e.actor)
		e.actor.Push(room.Stop{})
	}
	clients := make([]Client, 0, len(l.clients))
	for _, m := range l.clients {
		clients = append(clients, m.client)
	}
	l.mu.Unlock()
	var err error
	for _, a := range actors {
		select {
		case <-a.Done():
		case <-ctx.Done():
			err = ctx.Err()
		}
		if err != nil {
			break
		}
	}
	for _, c := range clients {
		c.Close(CloseShutdown)
	}
	l.cancel()
	return err
}

func (l *Lobby) roomOptions(id, code, name string, public bool) room.Options {
	return room.Options{ID: id, Code: code, Name: name, Public: public, Timing: l.opt.Timing, Store: l.opt.Store,
		Out: l, Registry: l, Clock: l.opt.Clock, Rand: l.rand, Log: l.opt.Log.With("room_id", id)}
}

func (l *Lobby) roomOfLocked(userID string) *entry {
	if id, ok := l.index[userID]; ok {
		return l.rooms[id]
	}
	return nil
}

func (l *Lobby) snapshotLocked() protocol.ServerMessage {
	var rooms []protocol.RoomSummary
	for _, e := range l.rooms {
		if e.summary.Public {
			rooms = append(rooms, e.summary.Room)
		}
	}
	slices.SortFunc(rooms, func(a, b protocol.RoomSummary) int { return strings.Compare(a.ID, b.ID) })
	return protocol.LobbySnapshot(rooms)
}

// broadcastLobbyLocked envia a quem está conectado e fora de sala.
func (l *Lobby) broadcastLobbyLocked(msg protocol.ServerMessage) {
	for id, m := range l.clients {
		if _, in := l.index[id]; !in {
			m.client.Send(msg)
		}
	}
}

const codeAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

// newIdentityLocked sorteia o id (UUID v4) e um código de 6 caracteres livre.
func (l *Lobby) newIdentityLocked() (id, code string, err error) {
	var b [16]byte
	if _, err := io.ReadFull(l.rand, b[:]); err != nil {
		return "", "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	id = fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
	for {
		var raw [6]byte
		if _, err := io.ReadFull(l.rand, raw[:]); err != nil {
			return "", "", err
		}
		buf := make([]byte, len(raw))
		for i, v := range raw {
			buf[i] = codeAlphabet[int(v)%len(codeAlphabet)]
		}
		if _, taken := l.codes[string(buf)]; !taken {
			return id, string(buf), nil
		}
	}
}

var (
	_ room.Outbox   = (*Lobby)(nil)
	_ room.Registry = (*Lobby)(nil)
)
