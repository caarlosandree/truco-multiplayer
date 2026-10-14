package engine

import "testing"

// vira 4♦ → manilha 5 (nenhuma nas mãos). Turno inicial: assento 1.
func playingState(hands [4][]Card, kind HandKind) GameState {
	vira := c(4, Diamonds)
	g := GameState{Seed: 1, HandNumber: 1, Players: testPlayers, Dealer: 0, Phase: PhasePlaying}
	stake := 1
	if kind == HandEleven {
		stake = 3
	}
	g.Hand = &HandState{Kind: kind, Hands: hands, Vira: &vira, Manilha: ManilhaRank(vira), Stake: stake, Turn: 1, RoundStarter: 1}
	return g
}

// Com slots 0 e depois 1, a dupla B (assento 1) vence duas rodadas.
func fixtureBWinsTwice() [4][]Card {
	return [4][]Card{
		{c(4, Clubs), c(6, Clubs), c(7, Clubs)},
		{c(3, Hearts), c(2, Hearts), c(1, Hearts)},
		{c(4, Spades), c(6, Spades), c(7, Spades)},
		{c(4, Hearts), c(6, Hearts), c(7, Hearts)},
	}
}

// Slot 0: empate (7 contra 7). Slot 1: dupla A vence com 3♣.
func fixtureTieThenA() [4][]Card {
	return [4][]Card{
		{c(7, Clubs), c(3, Clubs), c(4, Clubs)},
		{c(7, Hearts), c(2, Hearts), c(4, Hearts)},
		{c(6, Spades), c(1, Spades), c(4, Spades)},
		{c(6, Hearts), c(1, Hearts), c(7, Spades)},
	}
}

func playRound(t *testing.T, g GameState, slot int) GameState {
	t.Helper()
	for range 4 {
		g, _ = mustApply(t, g, Action{Kind: ActPlayCard, Seat: g.Hand.Turn, Slot: slot})
	}
	return g
}

func TestPlayCardRotatesTurn(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	_, _, err := Apply(g, Action{Kind: ActPlayCard, Seat: 2, Slot: 0})
	wantCode(t, err, ErrNotYourTurn)
	g, evs := mustApply(t, g, Action{Kind: ActPlayCard, Seat: 1, Slot: 0})
	if g.Hand.Turn != 2 || len(g.Hand.Table) != 1 || evs[0].Card == nil {
		t.Fatalf("vez=%d mesa=%d", g.Hand.Turn, len(g.Hand.Table))
	}
}

func TestRoundWinnerStartsNextRoundAndPlayedSlotCannotBeReused(t *testing.T) {
	g := playRound(t, playingState(fixtureBWinsTwice(), HandNormal), 0)
	if g.Hand.Turn != 1 || g.Hand.RoundStarter != 1 || len(g.Hand.Rounds) != 1 {
		t.Fatalf("vez=%d rodadas=%d", g.Hand.Turn, len(g.Hand.Rounds))
	}
	_, _, err := Apply(g, Action{Kind: ActPlayCard, Seat: 1, Slot: 0})
	wantCode(t, err, ErrInvalidSlot)
	g = playRound(t, g, 1)
	if g.Score != [2]int{0, 1} || g.Phase != PhaseHandEnd {
		t.Fatalf("placar=%v fase=%s", g.Score, g.Phase)
	}
}

func TestTiedRoundRestartsWithSameStarter(t *testing.T) {
	g := playRound(t, playingState(fixtureTieThenA(), HandNormal), 0)
	if g.Hand.Rounds[0].Winner != nil || g.Hand.Turn != 1 {
		t.Fatal("rodada empatada deve recomeçar por quem a abriu")
	}
	g = playRound(t, g, 1)
	if g.Score != [2]int{1, 0} || g.Phase != PhaseHandEnd {
		t.Fatalf("empate + vitória da A: placar=%v", g.Score)
	}
}

func TestCoveredCardLosesAndHidesIdentityInEvent(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	g, evs := mustApply(t, g, Action{Kind: ActPlayCard, Seat: 1, Slot: 0, Covered: true})
	if evs[0].Card != nil {
		t.Fatal("evento de carta coberta não pode trazer a carta")
	}
	g, _ = mustApply(t, g, Action{Kind: ActPlayCard, Seat: 2, Slot: 0})
	g, _ = mustApply(t, g, Action{Kind: ActPlayCard, Seat: 3, Slot: 1})
	g, _ = mustApply(t, g, Action{Kind: ActPlayCard, Seat: 0, Slot: 0})
	if *g.Hand.Rounds[0].WinnerSeat != 3 {
		t.Fatalf("vencedor = %d, quer 3 (6♥ vence 4s; 3♥ coberto vale 0)", *g.Hand.Rounds[0].WinnerSeat)
	}
}

func TestCoveredNotAllowedInIronHand(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandIron)
	_, _, err := Apply(g, Action{Kind: ActPlayCard, Seat: 1, Slot: 0, Covered: true})
	wantCode(t, err, ErrCoveredNotAllowed)
}

func TestReachingTwelveEndsGameAndBlocksFurtherActions(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	g.Score = [2]int{0, 11}
	g = playRound(t, playRound(t, g, 0), 1)
	if g.Phase != PhaseGameOver || g.Winner == nil || *g.Winner != 1 || g.Score[1] != 12 {
		t.Fatalf("fase=%s placar=%v", g.Phase, g.Score)
	}
	_, _, err := Apply(g, Action{Kind: ActSysStartHand})
	wantCode(t, err, ErrGameOver)
}
