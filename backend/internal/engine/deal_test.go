package engine

import (
	"slices"
	"testing"
)

func TestStartHandOffersTwoCardsToCutter(t *testing.T) {
	g, evs := mustApply(t, newTestGame(7), Action{Kind: ActSysStartHand})
	if g.Phase != PhaseCutChoice || g.Cutter() != 3 {
		t.Fatalf("fase = %s, cutter = %d", g.Phase, g.Cutter())
	}
	if len(g.Hand.CutOffer) != 2 || len(g.Hand.Deck) != 38 {
		t.Fatalf("oferta = %d, monte = %d", len(g.Hand.CutOffer), len(g.Hand.Deck))
	}
	if g.Hand.Kind != HandNormal || g.Hand.Stake != 1 || g.HandNumber != 1 {
		t.Fatalf("mão inesperada: kind=%s stake=%d n=%d", g.Hand.Kind, g.Hand.Stake, g.HandNumber)
	}
	for _, e := range evs {
		if e.Kind == EvCutOffered && !(e.Visibility.Allows(3) && !e.Visibility.Allows(0)) {
			t.Fatal("CutOffered deve ser visível só para quem corta")
		}
	}
}

func TestChooseCardRoutesCardsToPartners(t *testing.T) {
	g, _ := mustApply(t, newTestGame(7), Action{Kind: ActSysStartHand})
	offer := slices.Clone(g.Hand.CutOffer)
	g, _ = mustApply(t, g, Action{Kind: ActChooseCard, Seat: 3, Keep: 1})
	if g.Hand.Hands[3][0] != offer[1] || g.Hand.Hands[1][0] != offer[0] {
		t.Fatal("cortador fica com a escolhida e a outra vai para dealer+1")
	}
	if g.Phase != PhaseDealerChoice {
		t.Fatalf("fase = %s", g.Phase)
	}
	offer2 := slices.Clone(g.Hand.CutOffer)
	g, _ = mustApply(t, g, Action{Kind: ActChooseCard, Seat: 0, Keep: 0})
	if g.Hand.Hands[0][0] != offer2[0] || g.Hand.Hands[2][0] != offer2[1] {
		t.Fatal("dealer fica com a escolhida e a outra vai para dealer+2")
	}
	for s, hand := range g.Hand.Hands {
		if len(hand) != 3 {
			t.Fatalf("assento %d tem %d cartas", s, len(hand))
		}
	}
	assertConserved(t, g)
}

func TestChooseCardRejectsWrongSeatAndKeep(t *testing.T) {
	g, _ := mustApply(t, newTestGame(7), Action{Kind: ActSysStartHand})
	_, _, err := Apply(g, Action{Kind: ActChooseCard, Seat: 0, Keep: 0})
	wantCode(t, err, ErrNotYourTurn)
	_, _, err = Apply(g, Action{Kind: ActChooseCard, Seat: 3, Keep: 2})
	wantCode(t, err, ErrInvalidChoice)
	if g.Phase != PhaseCutChoice || len(g.Hand.CutOffer) != 2 {
		t.Fatal("estado original não pode mudar em erro")
	}
}

func TestDealRemainingStartsLeftOfDealer(t *testing.T) {
	h := &HandState{}
	for id := range 8 {
		h.Deck = append(h.Deck, CardFromID(CardID(id)))
	}
	for s := range h.Hands {
		h.Hands[s] = []Card{CardFromID(CardID(30 + s))}
	}
	dealRemaining(h, 0)
	// topo = último; ordem: 1,2,3,0,1,2,3,0
	want := map[Seat][2]CardID{1: {7, 3}, 2: {6, 2}, 3: {5, 1}, 0: {4, 0}}
	for s, ids := range want {
		if h.Hands[s][1].ID() != ids[0] || h.Hands[s][2].ID() != ids[1] {
			t.Fatalf("assento %d recebeu %v", s, h.Hands[s])
		}
	}
}

func burnFixture() [4][]Card {
	return [4][]Card{
		{c(4, Diamonds), c(7, Clubs), c(1, Hearts)},
		{c(5, Diamonds), c(6, Diamonds), c(7, Diamonds)},
		{c(5, Spades), c(6, Spades), c(7, Spades)},
		{c(5, Hearts), c(6, Hearts), c(7, Hearts)},
	}
}

func burnState(hands [4][]Card, deck []Card) GameState {
	g := GameState{Seed: 1, HandNumber: 1, Players: testPlayers, Dealer: 0}
	g.Hand = &HandState{Kind: HandNormal, Stake: 1, Hands: hands, Deck: deck}
	enterBurn(&g)
	return g
}

func TestBurnReplacesFourInSameSlotAndRevealsVira(t *testing.T) {
	g := burnState(burnFixture(), []Card{c(3, Clubs), c(2, Diamonds)})
	if g.Phase != PhaseBurnFours || !g.Hand.BurnDecided[1] || g.Hand.BurnDecided[0] {
		t.Fatalf("fase = %s, decididos = %v", g.Phase, g.Hand.BurnDecided)
	}
	g, evs := mustApply(t, g, Action{Kind: ActBurn, Seat: 0, Slots: []int{0}})
	if g.Hand.Hands[0][0] != c(2, Diamonds) || !slices.Equal(g.Hand.Discarded, []Card{c(4, Diamonds)}) {
		t.Fatalf("mão = %v, descarte = %v", g.Hand.Hands[0], g.Hand.Discarded)
	}
	// Risco tático: a vira 3 faz do 4 queimado uma manilha.
	if *g.Hand.Vira != c(3, Clubs) || g.Hand.Manilha != 4 {
		t.Fatalf("vira = %v, manilha = %d", *g.Hand.Vira, g.Hand.Manilha)
	}
	if g.Phase != PhasePlaying || g.Hand.Turn != 1 || g.Hand.RoundStarter != 1 {
		t.Fatalf("fase = %s, vez = %d", g.Phase, g.Hand.Turn)
	}
	if !hasEvent(evs, EvFoursBurned) || !hasEvent(evs, EvViraRevealed) {
		t.Fatal("faltam eventos de queima e vira")
	}
}

func TestPassBurnKeepsHand(t *testing.T) {
	g := burnState(burnFixture(), []Card{c(3, Clubs), c(2, Diamonds)})
	g, _ = mustApply(t, g, Action{Kind: ActPassBurn, Seat: 0})
	if g.Hand.Hands[0][0] != c(4, Diamonds) || *g.Hand.Vira != c(2, Diamonds) || len(g.Hand.Deck) != 1 {
		t.Fatal("passar não pode alterar a mão; vira deve ser o topo")
	}
}

func TestBurnRejectsInvalidInput(t *testing.T) {
	g := burnState(burnFixture(), []Card{c(3, Clubs), c(2, Diamonds), c(2, Spades)})
	_, _, err := Apply(g, Action{Kind: ActBurn, Seat: 0, Slots: []int{1}})
	wantCode(t, err, ErrBurnNotFour)
	_, _, err = Apply(g, Action{Kind: ActBurn, Seat: 0, Slots: []int{0, 0}})
	wantCode(t, err, ErrInvalidSlot)
	_, _, err = Apply(g, Action{Kind: ActBurn, Seat: 0, Slots: []int{5}})
	wantCode(t, err, ErrInvalidSlot)
	_, _, err = Apply(g, Action{Kind: ActBurn, Seat: 0, Slots: nil})
	wantCode(t, err, ErrInvalidSlot)
	_, _, err = Apply(g, Action{Kind: ActPassBurn, Seat: 1})
	wantCode(t, err, ErrAlreadyDecided)
}

func TestNoFoursSkipsBurnPhase(t *testing.T) {
	hands := burnFixture()
	hands[0][0] = c(3, Diamonds)
	g := burnState(hands, []Card{c(3, Clubs)})
	if g.Phase != PhasePlaying || g.Hand.Vira == nil {
		t.Fatalf("sem 4 na mesa a queima deve ser pulada; fase = %s", g.Phase)
	}
}

func TestReplacementFourCannotBeBurnedAgain(t *testing.T) {
	g := burnState(burnFixture(), []Card{c(3, Clubs), c(4, Clubs)})
	g, _ = mustApply(t, g, Action{Kind: ActBurn, Seat: 0, Slots: []int{0}})
	if g.Hand.Hands[0][0] != c(4, Clubs) || g.Phase != PhasePlaying {
		t.Fatal("o 4 comprado fica na mão e o jogo segue")
	}
}

func TestSimultaneousBurnsResolveClockwiseFromDealerLeft(t *testing.T) {
	hands := burnFixture()
	hands[2][0] = c(4, Spades)
	g := burnState(hands, []Card{c(3, Clubs), c(2, Diamonds), c(2, Spades)})
	g, _ = mustApply(t, g, Action{Kind: ActBurn, Seat: 2, Slots: []int{0}})
	if g.Phase != PhaseBurnFours {
		t.Fatal("deve aguardar a decisão do assento 0")
	}
	g, _ = mustApply(t, g, Action{Kind: ActBurn, Seat: 0, Slots: []int{0}})
	if g.Hand.Hands[2][0] != c(2, Spades) || g.Hand.Hands[0][0] != c(2, Diamonds) {
		t.Fatal("assento 2 resolve antes do assento 0 (ordem a partir de dealer+1)")
	}
	if !slices.Equal(g.Hand.Discarded, []Card{c(4, Spades), c(4, Diamonds)}) {
		t.Fatalf("descarte = %v", g.Hand.Discarded)
	}
}

func TestIronHandDealsBlindWithoutCutOrBurn(t *testing.T) {
	g := newTestGame(9)
	g.Score = [2]int{11, 11}
	g, _ = mustApply(t, g, Action{Kind: ActSysStartHand})
	if g.Hand.Kind != HandIron || g.Phase != PhasePlaying || g.Hand.Stake != 1 {
		t.Fatalf("kind=%s fase=%s stake=%d", g.Hand.Kind, g.Phase, g.Hand.Stake)
	}
	if g.Hand.CutOffer != nil || g.Hand.Vira == nil || len(g.Hand.Deck) != 27 {
		t.Fatal("ferro: sem corte, vira revelada, 13 cartas fora do monte")
	}
	assertConserved(t, g)
}

func TestElevenHandGoesToDecision(t *testing.T) {
	g := newTestGame(11)
	g.Score = [2]int{11, 5}
	g, _ = mustApply(t, g, Action{Kind: ActSysStartHand})
	g = dealWithoutBurning(t, g)
	if g.Phase != PhaseElevenDecision || g.Hand.Stake != 3 || *g.Hand.VotingTeam != 0 {
		t.Fatalf("fase=%s stake=%d", g.Phase, g.Hand.Stake)
	}
}

func TestDealerRotatesEachHand(t *testing.T) {
	g, _ := mustApply(t, newTestGame(3), Action{Kind: ActSysStartHand})
	g.Phase = PhaseHandEnd
	g, _ = mustApply(t, g, Action{Kind: ActSysStartHand})
	if g.Dealer != 1 || g.HandNumber != 2 || g.Cutter() != 0 {
		t.Fatalf("dealer=%d mão=%d", g.Dealer, g.HandNumber)
	}
}

func TestStartHandMidHandFails(t *testing.T) {
	g, _ := mustApply(t, newTestGame(3), Action{Kind: ActSysStartHand})
	_, _, err := Apply(g, Action{Kind: ActSysStartHand})
	wantCode(t, err, ErrInvalidPhase)
}

func TestApplyRejectsSeatOutOfRange(t *testing.T) {
	g, _ := mustApply(t, newTestGame(3), Action{Kind: ActSysStartHand})
	_, _, err := Apply(g, Action{Kind: ActChooseCard, Seat: 7})
	wantCode(t, err, ErrInvalidSeat)
}

func TestFilterEventsByVisibility(t *testing.T) {
	evs := []Event{
		{Kind: EvHandStarted, Visibility: visAll},
		{Kind: EvCutOffered, Visibility: visSeat(3)},
		{Kind: EvVoteCast, Visibility: visTeam(0)},
	}
	if got := FilterEvents(evs, 2); len(got) != 2 || got[1].Kind != EvVoteCast {
		t.Fatalf("assento 2 vê %v", got)
	}
	if got := FilterEvents(evs, 3); len(got) != 2 || got[1].Kind != EvCutOffered {
		t.Fatalf("assento 3 vê %v", got)
	}
}

func assertConserved(t *testing.T, g GameState) {
	t.Helper()
	cards := allCards(g)
	seen := map[Card]bool{}
	for _, card := range cards {
		if seen[card] {
			t.Fatalf("carta duplicada: %+v", card)
		}
		seen[card] = true
	}
	if len(cards) != DeckSize {
		t.Fatalf("%d cartas físicas, quer %d", len(cards), DeckSize)
	}
}
