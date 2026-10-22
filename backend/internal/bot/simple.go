package bot

import (
	"context"
	"math/rand/v2"
	"slices"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// Simple é a política de referência fixada (spec §5.5): joga a carta mais fraca
// que ganha a rodada, senão a mais fraca; nunca cobre, nunca queima, nunca
// blefa e não usa sinais; pede truco só com manilha (e nunca onde o pedido seria
// punido); aceita aumento com manilha ou duas cartas de rank A, 2 ou 3, senão
// corre; na mão de 11 joga só com manilha. Serve de rollout, fallback e
// adversário do benchmark: NÃO deve ser ajustada sem aval do usuário.
//
// Decisões não cobertas pela spec (corte, escolha do dealer) usam a primeira
// ação legal, que é determinística.
type Simple struct{}

var _ Policy = Simple{}

// Decide devolve sempre uma ação de in.Legal (ou Decision vazia se Legal é vazio).
func (Simple) Decide(_ context.Context, in Input, _ *rand.Rand) Decision {
	if len(in.Legal) == 0 {
		return Decision{}
	}
	return Decision{Action: simpleAction(in)}
}

func simpleAction(in Input) engine.Action {
	v := in.View
	switch v.Phase {
	case engine.PhaseBurnFours:
		if a, ok := findLegal(in.Legal, func(a engine.Action) bool { return a.Kind == engine.ActPassBurn }); ok {
			return a
		}
	case engine.PhaseElevenDecision:
		want := engine.VoteRun
		if hasManilha(v) {
			want = engine.VotePlay
		}
		if a, ok := findVote(in.Legal, want); ok {
			return a
		}
	case engine.PhaseAwaitingRaiseResponse:
		want := engine.VoteRun
		if hasManilha(v) || highCards(v) >= 2 {
			want = engine.VoteAccept
		}
		if a, ok := findVote(in.Legal, want); ok {
			return a
		}
	case engine.PhasePlaying:
		if a, ok := simplePlaying(in); ok {
			return a
		}
	}
	return in.Legal[0]
}

func simplePlaying(in Input) (engine.Action, bool) {
	v := in.View
	if v.HandKind == engine.HandNormal && hasManilha(v) {
		if a, ok := findLegal(in.Legal, func(a engine.Action) bool { return a.Kind == engine.ActRequestRaise }); ok {
			return a, true
		}
	}
	target := bestOpponentOnTable(v)
	var (
		bestWin, bestAny       engine.Action
		haveWin, haveAny       bool
		bestWinStr, bestAnyStr int
	)
	for _, a := range in.Legal {
		if a.Kind != engine.ActPlayCard || a.Covered {
			continue
		}
		s := slotStrength(v, a.Slot)
		if !haveAny || s < bestAnyStr {
			bestAny, bestAnyStr, haveAny = a, s, true
		}
		if s > target && (!haveWin || s < bestWinStr) {
			bestWin, bestWinStr, haveWin = a, s, true
		}
	}
	if haveWin {
		return bestWin, true
	}
	return bestAny, haveAny
}

// slotStrength é a força da carta do slot; carta oculta (mão de ferro) vale 0.
func slotStrength(v engine.PlayerView, slot int) int {
	for _, hs := range v.MyHand {
		if hs.Slot == slot && hs.Card != nil {
			return engine.Strength(engine.CardFromID(*hs.Card), false, manilhaRank(v))
		}
	}
	return 0
}

// bestOpponentOnTable é a maior força jogada pelos adversários nesta rodada
// (carta coberta alheia vale 0); -1 se nenhum adversário jogou ainda.
func bestOpponentOnTable(v engine.PlayerView) int {
	best := -1
	for _, t := range v.Table {
		if engine.TeamOf(t.Seat) == engine.TeamOf(v.Seat) {
			continue
		}
		s := 0
		if t.Card != nil && !t.Covered {
			s = engine.Strength(engine.CardFromID(*t.Card), false, manilhaRank(v))
		}
		best = max(best, s)
	}
	return best
}

func manilhaRank(v engine.PlayerView) engine.Rank {
	if v.Manilha == nil {
		return 0
	}
	return *v.Manilha
}

func hasManilha(v engine.PlayerView) bool {
	for _, hs := range v.MyHand {
		if hs.Card != nil && v.Manilha != nil && engine.CardFromID(*hs.Card).Rank == *v.Manilha {
			return true
		}
	}
	return false
}

// highCards conta as cartas não jogadas de rank A, 2 ou 3 (as três mais fortes
// fora a manilha).
func highCards(v engine.PlayerView) int {
	n := 0
	for _, hs := range v.MyHand {
		if hs.Card == nil {
			continue
		}
		if r := engine.CardFromID(*hs.Card).Rank; r == 1 || r == 2 || r == 3 {
			n++
		}
	}
	return n
}

func findLegal(legal []engine.Action, pred func(engine.Action) bool) (engine.Action, bool) {
	for _, a := range legal {
		if pred(a) {
			return a, true
		}
	}
	return engine.Action{}, false
}

func findVote(legal []engine.Action, c engine.VoteChoice) (engine.Action, bool) {
	return findLegal(legal, func(a engine.Action) bool { return a.Kind == engine.ActVote && a.Choice == c })
}

func sameAction(a, b engine.Action) bool {
	return a.Kind == b.Kind && a.Seat == b.Seat && a.Keep == b.Keep && a.Slot == b.Slot &&
		a.Covered == b.Covered && a.Choice == b.Choice && a.Team == b.Team && slices.Equal(a.Slots, b.Slots)
}
