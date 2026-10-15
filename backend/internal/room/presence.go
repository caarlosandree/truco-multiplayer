package room

import (
	"errors"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

// updateAbsence registra as transições de ausência e encurta o prazo de quem acabou de sumir.
func (r *Room) updateAbsence(now time.Time) {
	if r.status != protocol.RoomPlaying {
		return
	}
	changed, shortened := false, false
	for s, st := range r.seats {
		if st == nil {
			continue
		}
		seat := engine.Seat(s)
		was := !st.absentSince.IsZero()
		switch is := st.isAbsent(); {
		case is && !was:
			st.absentSince, changed = now, true
			if d, ok := r.deadlines[seat]; ok {
				if limit := now.Add(r.opt.Timing.Absent); limit.Before(d) {
					r.deadlines[seat], shortened = limit, true
				}
			}
		case !is && was:
			st.absentTotal += now.Sub(st.absentSince)
			st.absentSince, changed = time.Time{}, true
		}
	}
	if changed {
		r.publish()
	}
	if shortened {
		r.broadcastState()
	}
}

// forfeitAt é quando a ausência corrente do assento libera o W.O.: carência contínua
// ou orçamento acumulado na partida, o que vier antes.
func (r *Room) forfeitAt(st *seatState) time.Time {
	byGrace := st.absentSince.Add(r.opt.Timing.AbsenceGrace)
	byBudget := st.absentSince.Add(r.opt.Timing.AbsenceBudget - st.absentTotal)
	if byBudget.Before(byGrace) {
		return byBudget
	}
	return byGrace
}

// forfeitTarget é a dupla contra a qual o W.O. está disponível agora, se houver.
func (r *Room) forfeitTarget(now time.Time) *engine.Team {
	if r.status != protocol.RoomPlaying {
		return nil
	}
	var absentTeam [2]bool
	for s, st := range r.seats {
		if st != nil && !st.absentSince.IsZero() {
			absentTeam[engine.TeamOf(engine.Seat(s))] = true
		}
	}
	for s, st := range r.seats {
		if st == nil || st.absentSince.IsZero() {
			continue
		}
		team := engine.TeamOf(engine.Seat(s))
		if absentTeam[engine.Other(team)] {
			continue
		}
		if !now.Before(r.forfeitAt(st)) {
			return &team
		}
	}
	return nil
}

func sameTeam(a, b *engine.Team) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func (r *Room) sendToTeam(team engine.Team, msg protocol.ServerMessage) {
	for s, st := range r.seats {
		if st != nil && engine.TeamOf(engine.Seat(s)) == team {
			r.opt.Out.Send(st.userID, msg)
		}
	}
}

// offerForfeit avisa a dupla adversária quando o W.O. passa a valer ou deixa de valer.
func (r *Room) offerForfeit(now time.Time) {
	target := r.forfeitTarget(now)
	if sameTeam(target, r.offered) {
		return
	}
	previous := r.offered
	r.offered = target
	if previous != nil {
		r.sendToTeam(engine.Other(*previous), protocol.ForfeitAvailable(nil))
	}
	if target != nil {
		r.opt.Log.Info("W.O. disponível", "match_id", r.matchID, "team", *target)
		r.sendToTeam(engine.Other(*target), protocol.ForfeitAvailable(target))
	}
}

func (r *Room) claimForfeit(seat engine.Seat, seq int64) {
	user := r.seats[seat].userID
	if r.status != protocol.RoomPlaying || r.offered == nil || engine.TeamOf(seat) == *r.offered {
		r.reply(user, protocol.CodeForfeitUnavailable, seq)
		return
	}
	loser := *r.offered
	r.opt.Log.Info("W.O. pedido", "match_id", r.matchID, "loser", loser, "seat", seat)
	err := r.apply(engine.Action{Kind: engine.ActSysForfeit, Team: loser}, replay.SourceSystem)
	switch {
	case err == nil, errors.Is(err, errMatchGone):
	case errors.Is(err, errPersist):
		r.reply(user, protocol.CodeInternal, seq)
	default:
		r.opt.Out.Send(user, protocol.ErrorMessage(err, seq))
	}
}

// allAbsentAt é quando a partida deve ser abandonada por estarem todos ausentes.
func (r *Room) allAbsentAt() (time.Time, bool) {
	if r.status != protocol.RoomPlaying {
		return time.Time{}, false
	}
	var latest time.Time
	for _, st := range r.seats {
		if st == nil || st.absentSince.IsZero() {
			return time.Time{}, false
		}
		if st.absentSince.After(latest) {
			latest = st.absentSince
		}
	}
	return latest.Add(r.opt.Timing.AbsenceBudget), true
}

func (r *Room) abandonAllAbsent() {
	ctx, cancel := r.storeCtx()
	if err := r.opt.Store.Abandon(ctx, r.matchID, "all_absent"); err != nil && !errors.Is(err, ErrMatchClosed) {
		r.opt.Log.Error("falha ao abandonar partida sem jogadores", "match_id", r.matchID, "err", err)
	}
	cancel()
	r.opt.Log.Info("partida abandonada: todos ausentes", "match_id", r.matchID)
	ended := protocol.MatchEnded(protocol.EndAbandoned, nil, r.game.Score)
	for s, st := range r.seats {
		if st == nil {
			continue
		}
		r.opt.Out.Send(st.userID, ended)
		r.opt.Out.Send(st.userID, protocol.LeftRoom(protocol.LeftDestroyed))
		r.opt.Registry.Released(r.opt.ID, st.userID)
		r.seats[s] = nil
	}
	r.destroy()
}

// removeStaleDisconnected tira, em espera, quem está desconectado há WAITING_DISCONNECT.
func (r *Room) removeStaleDisconnected(now time.Time) bool {
	for s, st := range r.seats {
		if st != nil && !st.connected && !now.Before(st.disconnectedAt.Add(r.opt.Timing.WaitingDisconnect)) {
			r.removeSeat(engine.Seat(s), protocol.LeftLeft)
			return true
		}
	}
	return false
}
