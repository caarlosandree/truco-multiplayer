package engine

import (
	"errors"
	"testing"
)

var testPlayers = [4]Player{{ID: "p0", Name: "Ana"}, {ID: "p1", Name: "Bia"}, {ID: "p2", Name: "Caio"}, {ID: "p3", Name: "Duda"}}

func newTestGame(seed uint64) GameState { return NewGame(testPlayers, seed, 0) }

func mustApply(t *testing.T, g GameState, a Action) (GameState, []Event) {
	t.Helper()
	next, evs, err := Apply(g, a)
	if err != nil {
		t.Fatalf("Apply(%+v): %v", a, err)
	}
	return next, evs
}

func wantCode(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	var re *RuleError
	if !errors.As(err, &re) || re.Code != code {
		t.Fatalf("erro = %v, quer código %s", err, code)
	}
}

// allCards reúne todas as cartas físicas da mão (monte, oferta, mãos, descarte, vira).
func allCards(g GameState) []Card {
	h := g.Hand
	cards := append([]Card{}, h.Deck...)
	cards = append(cards, h.CutOffer...)
	for _, hand := range h.Hands {
		cards = append(cards, hand...)
	}
	cards = append(cards, h.Discarded...)
	if h.Vira != nil {
		cards = append(cards, *h.Vira)
	}
	return cards
}

// dealWithoutBurning faz as duas escolhas do corte (keep 0) e passa a queima de todos.
func dealWithoutBurning(t *testing.T, g GameState) GameState {
	t.Helper()
	g, _ = mustApply(t, g, Action{Kind: ActChooseCard, Seat: g.Cutter(), Keep: 0})
	g, _ = mustApply(t, g, Action{Kind: ActChooseCard, Seat: g.Dealer, Keep: 0})
	for s := Seat(0); s < 4 && g.Phase == PhaseBurnFours; s++ {
		if !g.Hand.BurnDecided[s] {
			g, _ = mustApply(t, g, Action{Kind: ActPassBurn, Seat: s})
		}
	}
	return g
}

func hasEvent(evs []Event, kind EventKind) bool {
	for _, e := range evs {
		if e.Kind == kind {
			return true
		}
	}
	return false
}
