package engine

import (
	"encoding/json"
	"math/rand/v2"
	"testing"
)

func TestRandomGamesKeepInvariantsAndNeverLeak(t *testing.T) {
	games := 500
	if testing.Short() {
		games = 100
	}
	for seed := range uint64(games) {
		playRandomGame(t, seed)
	}
}

func playRandomGame(t *testing.T, seed uint64) {
	t.Helper()
	rng := rand.New(rand.NewPCG(seed, 99))
	g := NewGame(testPlayers, seed, Seat(seed%4))
	for step := 0; g.Phase != PhaseGameOver; step++ {
		if step > 5000 {
			t.Fatalf("seed %d: partida não terminou", seed)
		}
		prevScore := g.Score
		var evs []Event
		g, evs = applyRandomLegal(t, g, rng, seed)
		checkInvariants(t, g, prevScore, seed)
		for s := Seat(0); s < 4; s++ {
			assertNoLeak(t, g, s, ViewFor(g, s), seed)
			assertNoLeak(t, g, s, FilterEvents(evs, s), seed)
		}
	}
}

func applyRandomLegal(t *testing.T, g GameState, rng *rand.Rand, seed uint64) (GameState, []Event) {
	t.Helper()
	candidates := legalCandidates(g)
	if len(candidates) == 0 {
		t.Fatalf("seed %d: nenhuma ação legal na fase %s", seed, g.Phase)
	}
	a := candidates[rng.IntN(len(candidates))]
	next, evs, err := Apply(g, a)
	if err != nil {
		t.Fatalf("seed %d: LegalActions listou ação recusada %+v: %v", seed, a, err)
	}
	return next, evs
}

func legalCandidates(g GameState) []Action {
	if g.Phase == PhaseWaiting || g.Phase == PhaseHandEnd {
		return []Action{{Kind: ActSysStartHand}}
	}
	var out []Action
	for s := Seat(0); s < 4; s++ {
		for _, a := range LegalActions(g, s) {
			// Truco só em mãos normais: a punição tem teste próprio e encerraria as partidas cedo demais.
			if a.Kind == ActRequestRaise && g.Hand.Kind != HandNormal {
				continue
			}
			out = append(out, a)
		}
		if _, ok := DefaultAction(g, s); ok {
			out = append(out, Action{Kind: ActSysTimeout, Seat: s})
		}
	}
	return out
}

func checkInvariants(t *testing.T, g GameState, prevScore [2]int, seed uint64) {
	t.Helper()
	for team := range 2 {
		if g.Score[team] < prevScore[team] || g.Score[team] > WinningScore {
			t.Fatalf("seed %d: placar inválido %v (antes %v)", seed, g.Score, prevScore)
		}
	}
	if g.Hand == nil {
		return
	}
	cards := allCards(g)
	seen := map[Card]bool{}
	for _, card := range cards {
		if seen[card] {
			t.Fatalf("seed %d: carta duplicada %+v", seed, card)
		}
		seen[card] = true
	}
	if len(cards) != DeckSize {
		t.Fatalf("seed %d: %d cartas físicas", seed, len(cards))
	}
}

// forbiddenCards lista o que o assento NÃO pode ver, sem usar a lógica de view.go.
func forbiddenCards(g GameState, viewer Seat) map[CardID]bool {
	forbidden := map[CardID]bool{}
	h := g.Hand
	if h == nil {
		return forbidden
	}
	add := func(card Card) { forbidden[card.ID()] = true }
	for _, card := range h.Deck {
		add(card)
	}
	for _, card := range h.Discarded {
		add(card)
	}
	viewerIsChoosing := (g.Phase == PhaseCutChoice && viewer == (g.Dealer+3)%4) ||
		(g.Phase == PhaseDealerChoice && viewer == g.Dealer)
	if !viewerIsChoosing {
		for _, card := range h.CutOffer {
			add(card)
		}
	}
	viewerTeamVoting := (g.Phase == PhaseAwaitingRaiseResponse || g.Phase == PhaseElevenDecision) &&
		h.VotingTeam != nil && *h.VotingTeam == TeamOf(viewer)
	for s := Seat(0); s < 4; s++ {
		for slot, card := range h.Hands[s] {
			if h.Played[s][slot] {
				continue // cartas jogadas são tratadas pela mesa
			}
			ownVisible := s == viewer && h.Kind != HandIron
			partnerVisible := s == Partner(viewer) && viewerTeamVoting
			if !ownVisible && !partnerVisible {
				add(card)
			}
		}
	}
	played := append([]PlayedCard{}, h.Table...)
	for _, round := range h.Rounds {
		played = append(played, round.Cards...)
	}
	for _, p := range played {
		if p.Covered && p.Seat != viewer {
			add(p.Card)
		}
	}
	return forbidden
}

// cardIDsInJSON coleta todo número sob as chaves "card" e "vira".
func cardIDsInJSON(t *testing.T, payload any) []CardID {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	var ids []CardID
	var walk func(key string, node any)
	walk = func(key string, node any) {
		switch n := node.(type) {
		case map[string]any:
			for k, child := range n {
				walk(k, child)
			}
		case []any:
			for _, child := range n {
				walk(key, child)
			}
		case float64:
			if key == "card" || key == "vira" {
				ids = append(ids, CardID(n))
			}
		}
	}
	walk("", doc)
	return ids
}

func assertNoLeak(t *testing.T, g GameState, viewer Seat, payload any, seed uint64) {
	t.Helper()
	forbidden := forbiddenCards(g, viewer)
	for _, id := range cardIDsInJSON(t, payload) {
		if forbidden[id] {
			t.Fatalf("seed %d, fase %s: assento %d recebeu carta proibida %+v", seed, g.Phase, viewer, CardFromID(id))
		}
	}
}
