package room

import (
	"errors"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

// settle registra ausências, executa o que já venceu (uma ação por volta, reavaliando a cada uma),
// atualiza a oferta de W.O. e arma um único timer para o próximo vencimento futuro.
func (r *Room) settle() {
	for !r.closed {
		now := r.now()
		r.updateAbsence(now)
		switch r.status {
		case protocol.RoomWaiting:
			if r.removeStaleDisconnected(now) {
				continue
			}
		case protocol.RoomPlaying:
			if at, all := r.allAbsentAt(); all && !now.Before(at) {
				r.abandonAllAbsent()
				return
			}
			if r.runDue(now) {
				continue
			}
		}
		r.offerForfeit(now)
		r.reschedule(now)
		return
	}
}

// runDue aplica a próxima ação automática vencida: início da mão após a pausa ou
// DefaultAction de um assento cujo prazo acabou. Devolve true se aplicou algo.
func (r *Room) runDue(now time.Time) bool {
	if r.game.Phase == engine.PhaseHandEnd {
		if now.Before(r.pauseUntil) {
			return false
		}
		err := r.apply(engine.Action{Kind: engine.ActSysStartHand}, replay.SourceSystem)
		if errors.Is(err, errPersist) {
			r.pauseUntil = now.Add(retryDelay)
		}
		return err == nil
	}
	for s := engine.Seat(0); s < 4; s++ {
		deadline, ok := r.deadlines[s]
		if !ok || now.Before(deadline) {
			continue
		}
		a, ok := engine.DefaultAction(r.game, s)
		if !ok {
			delete(r.deadlines, s)
			continue
		}
		r.opt.Log.Warn("timeout aplicado", "match_id", r.matchID, "seat", s, "action", a.Kind)
		err := r.apply(a, replay.SourceTimeout)
		switch {
		case errors.Is(err, errPersist):
			r.deadlines[s] = now.Add(retryDelay)
			return false
		case errors.Is(err, errMatchGone):
			return false
		case err != nil:
			r.opt.Log.Error("ação padrão rejeitada pelo engine", "match_id", r.matchID, "seat", s, "err", err)
			delete(r.deadlines, s)
			return false
		}
		if st := r.seats[s]; st != nil && r.status == protocol.RoomPlaying {
			st.timeouts++
		}
		return true
	}
	return false
}

func (r *Room) reschedule(now time.Time) {
	r.stopTimer()
	at, ok := r.nextWakeup(now)
	if !ok {
		return
	}
	r.gen++
	gen := r.gen
	r.timer = r.opt.Clock.AfterFunc(at.Sub(now), func() { r.opt.Post(tick{gen: gen}) })
}

// nextWakeup é o vencimento futuro mais próximo; o que já venceu foi tratado em settle.
func (r *Room) nextWakeup(now time.Time) (time.Time, bool) {
	var best time.Time
	found := false
	r.wakeups(func(t time.Time) {
		if t.After(now) && (!found || t.Before(best)) {
			best, found = t, true
		}
	})
	return best, found
}

// wakeups lista os instantes em que algo da sala vence.
func (r *Room) wakeups(consider func(time.Time)) {
	switch r.status {
	case protocol.RoomWaiting:
		for _, st := range r.seats {
			if st != nil && !st.connected {
				consider(st.disconnectedAt.Add(r.opt.Timing.WaitingDisconnect))
			}
		}
	case protocol.RoomPlaying:
		if r.game.Phase == engine.PhaseHandEnd {
			consider(r.pauseUntil)
		}
		for _, d := range r.deadlines {
			consider(d)
		}
		for _, st := range r.seats {
			if st != nil && !st.absentSince.IsZero() {
				consider(r.forfeitAt(st))
			}
		}
		if at, all := r.allAbsentAt(); all {
			consider(at)
		}
	}
}

func (r *Room) stopTimer() {
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
}
