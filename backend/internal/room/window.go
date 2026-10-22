package room

import (
	"fmt"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// absentAfterTimeouts timeouts seguidos tornam um jogador conectado ausente.
const absentAfterTimeouts = 3

// isAbsent: bot nunca fica ausente.
func (st *seatState) isAbsent() bool {
	return !st.bot && (st.left || !st.connected || st.timeouts >= absentAfterTimeouts)
}

// pendingSeats são os assentos com decisão pendente no estado.
func pendingSeats(g engine.GameState) []engine.Seat {
	var out []engine.Seat
	for s := engine.Seat(0); s < 4; s++ {
		if _, ok := engine.DefaultAction(g, s); ok {
			out = append(out, s)
		}
	}
	return out
}

// windowKey identifica a janela de decisão: muda com a vez, a rodada, a carta na mesa,
// o pedido de aumento e a fase, mas não quando um votante ou um queimador simultâneo decide.
func windowKey(g engine.GameState) string {
	h := g.Hand
	if h == nil {
		return string(g.Phase)
	}
	stake, voting := 0, -1
	if h.Pending != nil {
		stake = h.Pending.ToStake
	}
	if h.VotingTeam != nil {
		voting = int(*h.VotingTeam)
	}
	return fmt.Sprintf("%d|%s|%d|%d|%d|%d|%d", g.HandNumber, g.Phase, h.Turn, len(h.Rounds), len(h.Table), stake, voting)
}

func phaseDuration(t Timing, p engine.Phase) time.Duration {
	switch p {
	case engine.PhasePlaying:
		return t.Play
	case engine.PhaseAwaitingRaiseResponse, engine.PhaseElevenDecision:
		return t.Vote
	case engine.PhaseCutChoice, engine.PhaseDealerChoice:
		return t.Choice
	case engine.PhaseBurnFours:
		return t.Burn
	}
	return 0
}

// decisionTime é o prazo do assento na fase corrente; ausente decide em TIMER_ABSENT.
func (r *Room) decisionTime(s engine.Seat) time.Duration {
	d := phaseDuration(r.opt.Timing, r.game.Phase)
	if st := r.seats[s]; st != nil && st.isAbsent() && r.opt.Timing.Absent < d {
		d = r.opt.Timing.Absent
	}
	return d
}

// syncWindow rearma os prazos quando a janela muda; dentro da mesma janela só esquece
// quem já decidiu. Ao entrar em hand_end, marca o fim da pausa entre mãos.
func (r *Room) syncWindow(now time.Time) {
	key := windowKey(r.game)
	if key != r.window {
		r.window = key
		r.deadlines = map[engine.Seat]time.Time{}
		for _, s := range pendingSeats(r.game) {
			r.deadlines[s] = now.Add(r.decisionTime(s))
		}
		if r.game.Phase == engine.PhaseHandEnd {
			r.pauseUntil = now.Add(r.opt.Timing.HandPause)
		}
		return
	}
	pending := map[engine.Seat]bool{}
	for _, s := range pendingSeats(r.game) {
		pending[s] = true
	}
	for s := range r.deadlines {
		if !pending[s] {
			delete(r.deadlines, s)
		}
	}
}
