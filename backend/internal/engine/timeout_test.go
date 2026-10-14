package engine

import "testing"

func timeout(seat Seat) Action { return Action{Kind: ActSysTimeout, Seat: seat} }

func TestTimeoutInCutChoiceKeepsFirstCard(t *testing.T) {
	g, _ := mustApply(t, newTestGame(5), Action{Kind: ActSysStartHand})
	first := g.Hand.CutOffer[0]
	_, _, err := Apply(g, timeout(0))
	wantCode(t, err, ErrInvalidPhase) // o assento 0 não tem nada pendente
	g, _ = mustApply(t, g, timeout(3))
	if g.Phase != PhaseDealerChoice || g.Hand.Hands[3][0] != first {
		t.Fatal("timeout do corte mantém a primeira carta")
	}
}

func TestTimeoutInBurnPasses(t *testing.T) {
	g := burnState(burnFixture(), []Card{c(3, Clubs), c(2, Diamonds)})
	g, _ = mustApply(t, g, timeout(0))
	if g.Phase != PhasePlaying || g.Hand.Hands[0][0] != c(4, Diamonds) {
		t.Fatal("timeout na queima equivale a passar")
	}
}

func TestTimeoutInPlayPlaysLowestSlotCovered(t *testing.T) {
	g, _ := mustApply(t, playingState(fixtureBWinsTwice(), HandNormal), timeout(1))
	if pc := g.Hand.Table[0]; pc.Seat != 1 || !pc.Covered || pc.Card != c(3, Hearts) {
		t.Fatalf("mesa = %+v", g.Hand.Table)
	}
	iron, _ := mustApply(t, playingState(fixtureBWinsTwice(), HandIron), timeout(1))
	if iron.Hand.Table[0].Covered {
		t.Fatal("na mão de ferro o timeout joga aberta")
	}
}

func TestTimeoutInVoteRuns(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	g, _ = mustApply(t, g, Action{Kind: ActRequestRaise, Seat: 1})
	g, _ = mustApply(t, g, timeout(0))
	g, _ = mustApply(t, g, timeout(2))
	if g.Score != [2]int{0, 1} || g.Phase != PhaseHandEnd {
		t.Fatalf("placar=%v", g.Score)
	}
}
