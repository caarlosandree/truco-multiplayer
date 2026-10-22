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

// occupied conta os assentos ocupados (humanos e bots).
func (r *Room) occupied() int {
	n := 0
	for _, st := range r.seats {
		if st != nil {
			n++
		}
	}
	return n
}

// humans conta os assentos humanos: só eles mantêm a sala viva.
func (r *Room) humans() int {
	n := 0
	for _, st := range r.seats {
		if st != nil && !st.bot {
			n++
		}
	}
	return n
}

// firstHuman é o primeiro assento humano (o host nunca é bot).
func (r *Room) firstHuman() engine.Seat {
	for s, st := range r.seats {
		if st != nil && !st.bot {
			return engine.Seat(s)
		}
	}
	return 0
}

// isVacant: cadeira vazia ou de bot (um humano pode tomá-la).
func (r *Room) isVacant(s engine.Seat) bool { return r.seats[s] == nil || r.seats[s].bot }

func (r *Room) reply(userID, code string, seq int64) {
	r.opt.Out.Send(userID, protocol.ErrorMsg(code, seq))
}

func (r *Room) broadcast(msg protocol.ServerMessage) {
	for _, st := range r.seats {
		if st != nil && !st.bot {
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
		if m.Invite {
			fail(protocol.CodeInviteExpired)
		} else {
			fail(protocol.CodeMatchInProgress)
		}
		return
	}
	seat, code := r.pickSeat(m.Seat, m.Invite)
	if code != "" {
		fail(code)
		return
	}
	r.seats[seat] = &seatState{userID: m.UserID, name: m.DisplayName, connected: true} // substitui o bot, se houver
	if r.humans() == 1 {
		r.host = seat
	}
	r.opt.Registry.Seated(r.opt.ID, m.UserID)
	r.publish()
}

// vacate tira da sala de espera quem vai aceitar um convite para outra sala.
func (r *Room) vacate(m Vacate) {
	seat, ok := r.seatOf(m.UserID)
	switch {
	case !ok:
		r.opt.Registry.Released(r.opt.ID, m.UserID)
	case r.status != protocol.RoomWaiting:
		if m.Refused != nil {
			m.Refused()
		}
	default:
		r.removeSeat(seat, protocol.LeftLeft)
	}
}

func (r *Room) pickSeat(want *engine.Seat, invite bool) (engine.Seat, string) {
	if r.humans() == 4 {
		if invite {
			return 0, protocol.CodeInviteExpired
		}
		return 0, protocol.CodeRoomFull
	}
	if want != nil {
		if *want > 3 {
			return 0, protocol.CodeBadRequest
		}
		if !r.isVacant(*want) {
			if invite {
				return r.freeSeat(), ""
			}
			return 0, protocol.CodeSeatTaken
		}
		return *want, ""
	}
	return r.freeSeat(), ""
}

// freeSeat é o primeiro assento vazio; sem nenhum, o bot de menor número.
func (r *Room) freeSeat() engine.Seat {
	for s, st := range r.seats {
		if st == nil {
			return engine.Seat(s)
		}
	}
	for s, st := range r.seats {
		if st != nil && st.bot {
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
	case protocol.TypeStartWithBots:
		r.startWithBots(seat, m.Message)
	case protocol.TypeClaimForfeit:
		r.claimForfeit(seat, m.Message.Seq)
	case protocol.TypeSignal:
		r.signal(seat, m.Message)
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
	if !r.isVacant(p.Seat) {
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
	if !st.bot { // bot nunca entra no índice do lobby
		r.opt.Out.Send(st.userID, protocol.LeftRoom(reason))
		r.opt.Registry.Released(r.opt.ID, st.userID)
	}
	if r.humans() == 0 {
		clear(r.seats[:]) // bots não mantêm a sala viva
		r.destroy()
		return
	}
	if r.host == seat {
		r.host = r.firstHuman()
	}
	r.publish()
}

// startWithBots (só o host, em espera): senta um bot em cada cadeira vazia, marca o host pronto e
// deixa maybeStart decidir (a partida só começa com todos os humanos prontos).
func (r *Room) startWithBots(seat engine.Seat, msg protocol.ClientMessage) {
	user := r.seats[seat].userID
	switch {
	case r.status != protocol.RoomWaiting:
		r.reply(user, protocol.CodeNotWaiting, msg.Seq)
		return
	case seat != r.host:
		r.reply(user, protocol.CodeNotHost, msg.Seq)
		return
	}
	var empty []engine.Seat
	for s, st := range r.seats {
		if st == nil {
			empty = append(empty, engine.Seat(s))
		}
	}
	if len(empty) == 0 {
		r.reply(user, protocol.CodeNoEmptySeat, msg.Seq)
		return
	}
	free := r.freeBots()
	if len(free) < len(empty) {
		r.opt.Log.Error("roster de bots insuficiente", "precisa", len(empty), "tem", len(free))
		r.reply(user, protocol.CodeInternal, msg.Seq)
		return
	}
	for i, s := range empty {
		r.seats[s] = &seatState{userID: free[i].UserID, name: free[i].DisplayName, connected: true, ready: true, bot: true}
	}
	r.seats[seat].ready = true
	r.publish()
	r.maybeStart()
}

// freeBots são as identidades do roster ainda não sentadas nesta sala, em ordem de índice.
func (r *Room) freeBots() []BotIdentity {
	var out []BotIdentity
	for _, b := range r.opt.Bots {
		if _, seated := r.seatOf(b.UserID); !seated {
			out = append(out, b)
		}
	}
	return out
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
		if st != nil && !st.bot {
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
				Connected: st.connected, Bot: st.bot, Absent: r.status == protocol.RoomPlaying && !st.absentSince.IsZero(),
				BotPlaying: !st.bot && r.botControlled(engine.Seat(s))}
		}
	}
	if r.status == protocol.RoomPlaying {
		// O vencedor só se conhece no fim: ao vivo, só a partida que começou com bot já está fora do ranking.
		// O resultado definitivo vem em match_ended.ranked.
		ranked := !r.startedWithBots
		p.Ranked = &ranked
	}
	return protocol.Message(protocol.MsgRoomState, p)
}

func (r *Room) summary() Summary {
	rs := protocol.RoomSummary{ID: r.opt.ID, Name: r.opt.Name, Status: r.status}
	for s, st := range r.seats {
		if st != nil {
			rs.Seats[s] = &protocol.SeatSummary{DisplayName: st.name, Bot: st.bot}
		}
	}
	if r.status == protocol.RoomPlaying {
		score := r.game.Score
		rs.Score = &score
	}
	return Summary{Room: rs, Public: r.opt.Public, Code: r.opt.Code, MatchID: r.matchID}
}
