package engine

// HandWinner aplica a regra paulista de empates. decided=true com winner=nil
// significa que as três rodadas empataram e ninguém pontua.
func HandWinner(results []*Team) (winner *Team, decided bool) {
	var wins [2]int
	for _, r := range results {
		if r != nil {
			wins[*r]++
		}
	}
	for team := Team(0); team < 2; team++ {
		if wins[team] >= 2 {
			return teamPtr(team), true
		}
	}
	switch len(results) {
	case 2:
		first, second := results[0], results[1]
		if first != nil && second == nil {
			return clonePtr(first), true
		}
		if first == nil && second != nil {
			return clonePtr(second), true
		}
	case 3:
		// Casos restantes: (empate, empate, X) ou (X, Y, empate).
		if results[2] != nil {
			return clonePtr(results[2]), true
		}
		return clonePtr(results[0]), true
	}
	return nil, false
}

func playCard(g *GameState, a Action) ([]Event, error) {
	if g.Phase != PhasePlaying {
		return nil, ruleErr(ErrInvalidPhase, "não é hora de jogar carta")
	}
	h := g.Hand
	if a.Seat != h.Turn {
		return nil, ruleErr(ErrNotYourTurn, "não é sua vez")
	}
	if a.Slot < 0 || a.Slot >= len(h.Hands[a.Seat]) || h.Played[a.Seat][a.Slot] {
		return nil, ruleErr(ErrInvalidSlot, "slot inválido ou já jogado")
	}
	if a.Covered && h.Kind == HandIron {
		return nil, ruleErr(ErrCoveredNotAllowed, "não há carta coberta na mão de ferro")
	}
	card := h.Hands[a.Seat][a.Slot]
	h.Played[a.Seat][a.Slot] = true
	h.Table = append(h.Table, PlayedCard{Seat: a.Seat, Card: card, Covered: a.Covered})
	played := Event{Kind: EvCardPlayed, Visibility: visAll, Seat: seatPtr(a.Seat)}
	if !a.Covered {
		id := card.ID()
		played.Card = &id
	}
	evs := []Event{played}
	if len(h.Table) < 4 {
		h.Turn = Next(a.Seat)
		return evs, nil
	}
	return append(evs, finishRound(g)...), nil
}

func finishRound(g *GameState) []Event {
	h := g.Hand
	winner, winnerSeat := RoundWinner(h.Table, h.Manilha)
	h.Rounds = append(h.Rounds, RoundResult{Winner: winner, WinnerSeat: winnerSeat, Starter: h.RoundStarter, Cards: h.Table})
	h.Table = nil
	evs := []Event{{Kind: EvRoundEnded, Visibility: visAll, Team: clonePtr(winner), Seat: clonePtr(winnerSeat)}}
	results := make([]*Team, len(h.Rounds))
	for i, r := range h.Rounds {
		results[i] = r.Winner
	}
	if handWinner, decided := HandWinner(results); decided {
		return append(evs, endHand(g, handWinner, h.Stake)...)
	}
	nextStarter := h.RoundStarter // empate: recomeça quem abriu a rodada
	if winnerSeat != nil {
		nextStarter = *winnerSeat
	}
	h.RoundStarter, h.Turn = nextStarter, nextStarter
	return evs
}

func endHand(g *GameState, winner *Team, points int) []Event {
	if winner != nil {
		g.Score[*winner] = min(g.Score[*winner]+points, WinningScore)
	}
	score := g.Score
	evs := []Event{{Kind: EvHandEnded, Visibility: visAll, Team: clonePtr(winner), Stake: points, Score: &score}}
	g.Hand.Pending, g.Hand.VotingTeam = nil, nil
	if winner != nil && g.Score[*winner] >= WinningScore {
		return append(evs, finishGame(g, *winner)...)
	}
	g.Phase = PhaseHandEnd
	return evs
}

func finishGame(g *GameState, winner Team) []Event {
	g.Winner = teamPtr(winner)
	g.Phase = PhaseGameOver
	score := g.Score
	return []Event{{Kind: EvGameOver, Visibility: visAll, Team: teamPtr(winner), Score: &score}}
}
