package bot

import (
	"math"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// Belief guarda, para cada assento, um peso por carta não vista: quanto mais
// alto, mais provável que o assento tenha aquela carta. Os pesos vêm de
// evidências observáveis na mão atual (pedidos e votos de aumento, corrida e
// sinais; spec §5.1, §5.3 e §5.6). Cartas já vistas pelo assento do bot têm
// peso 0 (eliminadas). A crença nunca olha para o estado real: só para Input.
//
// O valor zero de Belief é válido e equivale a "sem informação": Sample cai
// para o sorteio uniforme sobre as cartas não vistas.
type Belief struct {
	w      [4][engine.DeckSize]float64
	str    [engine.DeckSize]float64
	unseen [engine.DeckSize]bool
}

// Intensidades das evidências. O peso de uma carta c para o assento s é
// exp(tilt[s] * z(c)), com z em [-1, 1] (força normalizada): tilt > 0 puxa para
// cartas fortes, tilt < 0 para fracas.
const (
	tiltRaiseSeat   = 1.5  // quem pediu o aumento
	tiltRaiseTeam   = 1.0  // dupla que re-aumentou (o voto é da dupla; assento incerto)
	tiltVoteStrong  = 1.0  // voto accept/raise/play visível
	tiltVoteRun     = -1.5 // voto de corrida visível (puxa para fracas)
	tiltAccepted    = 0.75 // dupla aceitou o aumento
	tiltElevenPlay  = 1.0  // dupla decidiu jogar a mão de 11
	tiltSigTruco    = 1.5  // sinal do parceiro: tenho mão forte
	tiltSigVamos    = 0.75
	tiltSigCalma    = -1.0
	opponentSigFrac = 0.25 // sinal de adversário vale uma fração do de parceiro (§5.6)
)

// NewBelief monta a crença do assento in.View.Seat a partir do Input.
func NewBelief(in Input) Belief {
	v := in.View
	var b Belief
	known := knownCards(v)
	manilha := manilhaRank(v)
	for id := range engine.DeckSize {
		b.unseen[id] = !known[id]
		b.str[id] = float64(engine.Strength(engine.CardFromID(engine.CardID(id)), false, manilha))
	}

	var tilt [4]float64
	me := v.Seat
	add := func(s *engine.Seat, t float64) {
		if s != nil && *s != me && *s < 4 {
			tilt[*s] += t
		}
	}
	addTeam := func(t *engine.Team, x float64) {
		if t == nil {
			return
		}
		for s := engine.Seat(0); s < 4; s++ {
			if engine.TeamOf(s) == *t {
				add(&s, x)
			}
		}
	}
	for _, e := range handEvents(in.Events) {
		switch e.Kind {
		case engine.EvRaiseRequested:
			if e.Seat != nil {
				add(e.Seat, tiltRaiseSeat)
			} else {
				addTeam(e.Team, tiltRaiseTeam)
			}
		case engine.EvVoteCast:
			if e.Choice == engine.VoteRun {
				add(e.Seat, tiltVoteRun)
			} else {
				add(e.Seat, tiltVoteStrong)
			}
		case engine.EvRaiseResolved:
			if e.Choice == engine.VoteAccept {
				addTeam(e.Team, tiltAccepted)
			}
		case engine.EvElevenDecided:
			if e.Choice == engine.VotePlay {
				addTeam(e.Team, tiltElevenPlay)
			}
		}
	}
	for _, sg := range in.Signals {
		if sg.HandNumber != v.HandNumber || sg.Seat == me || sg.Seat > 3 {
			continue
		}
		frac := opponentSigFrac
		if engine.TeamOf(sg.Seat) == engine.TeamOf(me) {
			frac = 1
		}
		switch sg.Signal {
		case SigTruco:
			tilt[sg.Seat] += frac * tiltSigTruco
		case SigVamos:
			tilt[sg.Seat] += frac * tiltSigVamos
		case SigCalma:
			tilt[sg.Seat] += frac * tiltSigCalma
		}
	}

	for s := range 4 {
		for id := range engine.DeckSize {
			if b.unseen[id] {
				b.w[s][id] = math.Exp(tilt[s] * (b.str[id] - 7.5) / 6.5)
			}
		}
	}
	return b
}

// handEvents devolve os eventos da mão atual (a partir do último hand_started).
func handEvents(evs []engine.Event) []engine.Event {
	for i := len(evs) - 1; i >= 0; i-- {
		if evs[i].Kind == engine.EvHandStarted {
			return evs[i:]
		}
	}
	return evs
}

// expectedStrength é a força esperada de uma carta sorteada para o assento
// segundo a crença (média ponderada pelos pesos sobre as cartas não vistas).
func (b Belief) expectedStrength(seat engine.Seat) float64 {
	var sum, total float64
	for id := range engine.DeckSize {
		w := b.w[seat][id]
		if !b.unseen[id] {
			continue
		}
		sum += w * b.str[id]
		total += w
	}
	if total == 0 {
		return 0
	}
	return sum / total
}
