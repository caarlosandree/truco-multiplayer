package room

import (
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
)

func (r *Room) seatOf(userID string) (engine.Seat, bool) {
	for s, st := range r.seats {
		if st != nil && st.userID == userID {
			return engine.Seat(s), true
		}
	}
	return 0, false
}

func (r *Room) members() int {
	n := 0
	for _, st := range r.seats {
		if st != nil {
			n++
		}
	}
	return n
}

func (r *Room) firstOccupied() engine.Seat {
	for s, st := range r.seats {
		if st != nil {
			return engine.Seat(s)
		}
	}
	return 0
}

func (r *Room) reply(userID, code string, seq int64) {
	r.opt.Out.Send(userID, protocol.ErrorMsg(code, seq))
}

func (r *Room) broadcast(msg protocol.ServerMessage) {
	for _, st := range r.seats {
		if st != nil {
			r.opt.Out.Send(st.userID, msg)
		}
	}
}

func (r *Room) join(m Join) {
	if seat, ok := r.seatOf(m.UserID); ok {
		r.opt.Out.Send(m.UserID, r.roomState(seat))
		return
	}
	fail := func(code string) {
		r.reply(m.UserID, code, m.Seq)
		r.opt.Registry.Released(r.opt.ID, m.UserID)
	}
	if r.status != protocol.RoomWaiting {
		fail(protocol.CodeMatchInProgress)
		return
	}
	seat, code := r.pickSeat(m.Seat)
	if code != "" {
		fail(code)
		return
	}
	r.seats[seat] = &seatState{userID: m.UserID, name: m.DisplayName, connected: true}
	if r.members() == 1 {
		r.host = seat
	}
	r.publish()
}

func (r *Room) pickSeat(want *engine.Seat) (engine.Seat, string) {
	if r.members() == 4 {
		return 0, protocol.CodeRoomFull
	}
	if want != nil {
		if *want > 3 {
			return 0, protocol.CodeBadRequest
		}
		if r.seats[*want] != nil {
			return 0, protocol.CodeSeatTaken
		}
		return *want, ""
	}
	return r.freeSeat(), ""
}

func (r *Room) freeSeat() engine.Seat {
	for s, st := range r.seats {
		if st == nil {
			return engine.Seat(s)
		}
	}
	return 0
}

func (r *Room) command(m Command) {
	seat, ok := r.seatOf(m.UserID)
	if !ok {
		r.reply(m.UserID, protocol.CodeNotInRoom, m.Message.Seq)
		return
	}
	switch m.Message.Type {
	case protocol.TypeLeaveRoom:
		r.leave(seat, m.Message)
	case protocol.TypeTakeSeat:
		r.takeSeat(seat, m.Message)
	case protocol.TypeSetReady:
		r.setReady(seat, m.Message)
	case protocol.TypeKick:
		r.kick(seat, m.Message)
	default:
		r.playerAction(seat, m.Message)
	}
}

// requireWaiting responde match_in_progress quando a sala está em jogo.
func (r *Room) requireWaiting(seat engine.Seat, seq int64) bool {
	if r.status == protocol.RoomWaiting {
		return true
	}
	r.reply(r.seats[seat].userID, protocol.CodeMatchInProgress, seq)
	return false
}

func (r *Room) leave(seat engine.Seat, msg protocol.ClientMessage) {
	if r.requireWaiting(seat, msg.Seq) {
		r.removeSeat(seat, protocol.LeftLeft)
	}
}

func (r *Room) takeSeat(seat engine.Seat, msg protocol.ClientMessage) {
	if !r.requireWaiting(seat, msg.Seq) {
		return
	}
	st := r.seats[seat]
	var p protocol.TakeSeatPayload
	if err := protocol.Decode(msg.Payload, &p, "seat"); err != nil || p.Seat > 3 {
		r.reply(st.userID, protocol.CodeBadRequest, msg.Seq)
		return
	}
	if p.Seat == seat {
		r.opt.Out.Send(st.userID, r.roomState(seat))
		return
	}
	if r.seats[p.Seat] != nil {
		r.reply(st.userID, protocol.CodeSeatTaken, msg.Seq)
		return
	}
	r.seats[p.Seat], r.seats[seat] = st, nil
	if r.host == seat {
		r.host = p.Seat
	}
	st.ready = false
	r.publish()
}

func (r *Room) setReady(seat engine.Seat, msg protocol.ClientMessage) {
	if !r.requireWaiting(seat, msg.Seq) {
		return
	}
	st := r.seats[seat]
	var p protocol.SetReadyPayload
	if err := protocol.Decode(msg.Payload, &p, "ready"); err != nil {
		r.reply(st.userID, protocol.CodeBadRequest, msg.Seq)
		return
	}
	st.ready = p.Ready
	r.publish()
	r.maybeStart()
}

func (r *Room) kick(seat engine.Seat, msg protocol.ClientMessage) {
	if !r.requireWaiting(seat, msg.Seq) {
		return
	}
	user := r.seats[seat].userID
	if seat != r.host {
		r.reply(user, protocol.CodeNotHost, msg.Seq)
		return
	}
	var p protocol.KickPayload
	if err := protocol.Decode(msg.Payload, &p, "seat"); err != nil || p.Seat > 3 || p.Seat == seat || r.seats[p.Seat] == nil {
		r.reply(user, protocol.CodeBadRequest, msg.Seq)
		return
	}
	r.removeSeat(p.Seat, protocol.LeftKicked)
}

// removeSeat tira o ocupante; a sala vazia é destruída.
func (r *Room) removeSeat(seat engine.Seat, reason protocol.LeftReason) {
	st := r.seats[seat]
	r.seats[seat] = nil
	r.opt.Out.Send(st.userID, protocol.LeftRoom(reason))
	r.opt.Registry.Released(r.opt.ID, st.userID)
	if r.members() == 0 {
		r.destroy()
		return
	}
	if r.host == seat {
		r.host = r.firstOccupied()
	}
	r.publish()
}

func (r *Room) destroy() {
	r.closed = true
	r.stopTimer()
	r.opt.Registry.Destroyed(r.opt.ID)
}

// connected marca o assento como conectado (nova conexão ou outra aba) e reenvia o que o cliente precisa.
func (r *Room) connected(userID string) {
	seat, ok := r.seatOf(userID)
	if !ok {
		return
	}
	st := r.seats[seat]
	st.connected, st.disconnectedAt, st.timeouts = true, time.Time{}, 0
	r.publish()
	if r.status != protocol.RoomPlaying {
		return
	}
	r.opt.Out.Send(userID, r.stateFor(seat))
	if r.offered != nil && engine.TeamOf(seat) != *r.offered {
		r.opt.Out.Send(userID, protocol.ForfeitAvailable(r.offered))
	}
}

func (r *Room) disconnected(userID string) {
	seat, ok := r.seatOf(userID)
	if !ok {
		return
	}
	st := r.seats[seat]
	st.connected, st.disconnectedAt = false, r.now()
	r.publish()
}

// publish envia o room_state a cada membro e o resumo ao lobby.
func (r *Room) publish() {
	for s, st := range r.seats {
		if st != nil {
			r.opt.Out.Send(st.userID, r.roomState(engine.Seat(s)))
		}
	}
	r.opt.Registry.Update(r.summary())
}

func (r *Room) visibility() protocol.Visibility {
	if r.opt.Public {
		return protocol.VisibilityPublic
	}
	return protocol.VisibilityPrivate
}

func (r *Room) roomState(seat engine.Seat) protocol.ServerMessage {
	p := protocol.RoomStatePayload{ID: r.opt.ID, Code: r.opt.Code, Name: r.opt.Name, Visibility: r.visibility(),
		Host: r.host, Status: r.status, MatchID: r.matchID, YourSeat: seat}
	for s, st := range r.seats {
		if st != nil {
			p.Seats[s] = &protocol.SeatInfo{UserID: st.userID, DisplayName: st.name, Ready: st.ready,
				Connected: st.connected, Absent: r.status == protocol.RoomPlaying && !st.absentSince.IsZero()}
		}
	}
	return protocol.Message(protocol.MsgRoomState, p)
}

func (r *Room) summary() Summary {
	rs := protocol.RoomSummary{ID: r.opt.ID, Name: r.opt.Name, Status: r.status}
	for s, st := range r.seats {
		if st != nil {
			rs.Seats[s] = &protocol.SeatSummary{DisplayName: st.name}
		}
	}
	if r.status == protocol.RoomPlaying {
		score := r.game.Score
		rs.Score = &score
	}
	return Summary{Room: rs, Public: r.opt.Public, Code: r.opt.Code, MatchID: r.matchID}
}
