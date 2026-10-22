package bot

import (
	"math/rand/v2"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// Frequências dos sinais (spec §5.6: o bot manda pouco).
const (
	intentSignalProb = 0.6  // quando a situação pede um sinal de intenção
	socialSignalProb = 0.2  // reações sociais, baixa frequência
	maxSignalsInHand = 3    // teto de sinais do bot numa mão (intenção + reações)
	apologyProb      = 0.35 // errar uma rodada decisiva pede desculpa mais vezes
)

// intentSignal decide o sinal que acompanha a decisão d do bot, ou nil. No
// máximo UM sinal de intenção (truco, vamos, calma) por mão, conforme a tabela
// de §5.6; as reações sociais (boa, valeu, desculpa) saem com baixa frequência,
// logo depois do evento que as motiva. Usa só o Input.
func intentSignal(in Input, d engine.Action, rng *rand.Rand) *Signal {
	v := in.View
	if v.Seat > 3 || v.HandKind == "" {
		return nil
	}
	sent, sentIntent := ownSignalsInHand(in)
	if sent >= maxSignalsInHand {
		return nil
	}
	if s := socialSignal(in, rng); s != nil {
		return s
	}
	if sentIntent || v.HandKind == engine.HandIron || v.Phase == engine.PhaseElevenDecision {
		return nil
	}
	var sig Signal
	strong, weak := strongHand(v), weakHand(v)
	switch {
	case d.Kind == engine.ActRequestRaise && strong:
		sig = SigTruco // pedi truco com mão forte
	case d.Kind == engine.ActPlayCard && v.Phase == engine.PhasePlaying && len(v.MyHand) == 3 && len(v.Table) == 0 && len(v.Rounds) == 0:
		// Primeira jogada da mão: avisa o parceiro se a mão é muito forte ou muito fraca.
		switch {
		case strong:
			sig = SigTruco
		case weak:
			sig = SigCalma
		}
	case d.Kind == engine.ActVote && v.Phase == engine.PhaseAwaitingRaiseResponse && (d.Choice == engine.VoteAccept || d.Choice == engine.VoteRaise) && strong:
		sig = SigVamos // aceitei com mão forte: peço que o parceiro siga
	}
	if sig == "" || rng.Float64() >= intentSignalProb {
		return nil
	}
	return &sig
}

// ownSignalsInHand conta os sinais que este assento já mandou na mão atual e
// diz se algum deles foi de intenção.
func ownSignalsInHand(in Input) (total int, intent bool) {
	for _, sg := range in.Signals {
		if sg.Seat != in.View.Seat || sg.HandNumber != in.View.HandNumber {
			continue
		}
		total++
		if sg.Signal == SigTruco || sg.Signal == SigVamos || sg.Signal == SigCalma {
			intent = true
		}
	}
	return total, intent
}

// socialSignal reage ao fim da última rodada, se ela terminou depois da última
// carta deste assento: boa (a dupla ganhou), valeu (o parceiro jogou a carta
// que ganhou) e desculpa (este assento jogou na rodada decisiva que a dupla
// perdeu). Cada rodada gera no máximo uma reação deste assento.
func socialSignal(in Input, rng *rand.Rand) *Signal {
	v := in.View
	me := v.Seat
	evs := handEvents(in.Events)
	var ended *engine.Event
	for i := len(evs) - 1; i >= 0; i-- {
		e := evs[i]
		if e.Kind == engine.EvCardPlayed && e.Seat != nil && *e.Seat == me {
			break
		}
		if e.Kind == engine.EvRoundEnded {
			ended = &evs[i]
			break
		}
	}
	if ended == nil || ended.Team == nil {
		return nil
	}
	// Uma reação por rodada: se já mandou tantos sociais quanto rodadas
	// terminadas, não manda de novo.
	if socialSent(in) >= len(v.Rounds) {
		return nil
	}
	myTeam := engine.TeamOf(me)
	var sig Signal
	prob := socialSignalProb
	switch {
	case *ended.Team == myTeam && ended.Seat != nil && *ended.Seat == engine.Partner(me):
		sig = SigValeu
	case *ended.Team == myTeam:
		sig = SigBoa
	case len(v.Rounds) == 2 && v.Rounds[0].Winner != nil && *v.Rounds[0].Winner == myTeam && playedIn(v.Rounds[1], me):
		sig, prob = SigDesculpa, apologyProb
	default:
		return nil
	}
	if rng.Float64() >= prob {
		return nil
	}
	return &sig
}

func socialSent(in Input) int {
	n := 0
	for _, sg := range in.Signals {
		if sg.Seat == in.View.Seat && sg.HandNumber == in.View.HandNumber &&
			(sg.Signal == SigBoa || sg.Signal == SigValeu || sg.Signal == SigDesculpa) {
			n++
		}
	}
	return n
}

func playedIn(r engine.RoundView, seat engine.Seat) bool {
	for _, c := range r.Cards {
		if c.Seat == seat {
			return true
		}
	}
	return false
}

// strongHand: manilha ou duas cartas de rank A, 2 ou 3 (mesma régua do Simple
// para aceitar aumento).
func strongHand(v engine.PlayerView) bool { return hasManilha(v) || highCards(v) >= 2 }

// weakHand: nenhuma manilha nem carta de rank A, 2 ou 3.
func weakHand(v engine.PlayerView) bool { return !hasManilha(v) && highCards(v) == 0 }
