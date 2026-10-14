package engine

import "slices"

func startHand(g *GameState) ([]Event, error) {
	if g.Phase != PhaseWaiting && g.Phase != PhaseHandEnd {
		return nil, ruleErr(ErrInvalidPhase, "há uma mão em andamento")
	}
	if g.HandNumber > 0 {
		g.Dealer = Next(g.Dealer)
	}
	g.HandNumber++
	deck := NewDeck()
	Shuffle(deck, handRNG(g.Seed, g.HandNumber))
	kind := handKindFor(g.Score)
	hand := &HandState{Kind: kind, Deck: deck, Stake: 1}
	if kind == HandEleven {
		hand.Stake = 3
	}
	g.Hand = hand
	evs := []Event{{Kind: EvHandStarted, Visibility: visAll, Seat: seatPtr(g.Dealer)}}
	if kind == HandIron {
		dealRemaining(hand, g.Dealer)
		evs = append(evs, Event{Kind: EvCardsDealt, Visibility: visAll})
		evs = append(evs, revealVira(g)...)
		return append(evs, beginAfterVira(g)...), nil
	}
	hand.CutOffer = []Card{draw(hand), draw(hand)}
	g.Phase = PhaseCutChoice
	cutter := g.Cutter()
	return append(evs, Event{Kind: EvCutOffered, Visibility: visSeat(cutter), Seat: seatPtr(cutter)}), nil
}

func handKindFor(score [2]int) HandKind {
	teamAOnEleven := score[0] == WinningScore-1
	teamBOnEleven := score[1] == WinningScore-1
	switch {
	case teamAOnEleven && teamBOnEleven:
		return HandIron
	case teamAOnEleven || teamBOnEleven:
		return HandEleven
	}
	return HandNormal
}

func draw(h *HandState) Card {
	top := h.Deck[len(h.Deck)-1]
	h.Deck = h.Deck[:len(h.Deck)-1]
	return top
}

// dealRemaining distribui a partir da esquerda do dealer até todos terem 3 cartas.
func dealRemaining(h *HandState, dealer Seat) {
	for s := Next(dealer); !allHaveThree(h); s = Next(s) {
		if len(h.Hands[s]) < 3 {
			h.Hands[s] = append(h.Hands[s], draw(h))
		}
	}
}

func allHaveThree(h *HandState) bool {
	for _, hand := range h.Hands {
		if len(hand) < 3 {
			return false
		}
	}
	return true
}

func chooseCard(g *GameState, a Action) ([]Event, error) {
	var chooser Seat
	switch g.Phase {
	case PhaseCutChoice:
		chooser = g.Cutter()
	case PhaseDealerChoice:
		chooser = g.Dealer
	default:
		return nil, ruleErr(ErrInvalidPhase, "não há carta para escolher")
	}
	if a.Seat != chooser {
		return nil, ruleErr(ErrNotYourTurn, "não é você quem escolhe")
	}
	if a.Keep != 0 && a.Keep != 1 {
		return nil, ruleErr(ErrInvalidChoice, "keep deve ser 0 ou 1")
	}
	h := g.Hand
	kept, given := h.CutOffer[a.Keep], h.CutOffer[1-a.Keep]
	h.Hands[chooser] = append(h.Hands[chooser], kept)
	h.Hands[Partner(chooser)] = append(h.Hands[Partner(chooser)], given)
	h.CutOffer = nil
	evs := []Event{{Kind: EvCardChosen, Visibility: visAll, Seat: seatPtr(chooser)}}
	if g.Phase == PhaseCutChoice {
		h.CutOffer = []Card{draw(h), draw(h)}
		g.Phase = PhaseDealerChoice
		return append(evs, Event{Kind: EvCutOffered, Visibility: visSeat(g.Dealer), Seat: seatPtr(g.Dealer)}), nil
	}
	dealRemaining(h, g.Dealer)
	evs = append(evs, Event{Kind: EvCardsDealt, Visibility: visAll})
	return append(evs, enterBurn(g)...), nil
}

func enterBurn(g *GameState) []Event {
	h := g.Hand
	for s := range h.Hands {
		h.BurnDecided[s] = !hasFour(h.Hands[s])
	}
	g.Phase = PhaseBurnFours
	if allBurnDecided(h) {
		return resolveBurns(g)
	}
	return nil
}

func hasFour(hand []Card) bool {
	return slices.ContainsFunc(hand, func(card Card) bool { return card.Rank == 4 })
}

func allBurnDecided(h *HandState) bool {
	return !slices.Contains(h.BurnDecided[:], false)
}

// burn trata ActBurn e ActPassBurn; decisões são simultâneas.
func burn(g *GameState, a Action) ([]Event, error) {
	if g.Phase != PhaseBurnFours {
		return nil, ruleErr(ErrInvalidPhase, "não é hora de queimar")
	}
	h := g.Hand
	if h.BurnDecided[a.Seat] {
		return nil, ruleErr(ErrAlreadyDecided, "queima já decidida")
	}
	var slots []int
	if a.Kind == ActBurn {
		if len(a.Slots) == 0 {
			return nil, ruleErr(ErrInvalidSlot, "nenhum slot informado")
		}
		seen := map[int]bool{}
		for _, slot := range a.Slots {
			if slot < 0 || slot >= len(h.Hands[a.Seat]) || seen[slot] {
				return nil, ruleErr(ErrInvalidSlot, "slot inválido ou repetido")
			}
			if h.Hands[a.Seat][slot].Rank != 4 {
				return nil, ruleErr(ErrBurnNotFour, "só é possível queimar 4")
			}
			seen[slot] = true
		}
		slots = slices.Clone(a.Slots)
	}
	h.BurnSlots[a.Seat] = slots
	h.BurnDecided[a.Seat] = true
	if allBurnDecided(h) {
		return resolveBurns(g), nil
	}
	return nil, nil
}

// resolveBurns troca os 4 em ordem horária a partir de dealer+1; não há nova queima.
func resolveBurns(g *GameState) []Event {
	h := g.Hand
	var evs []Event
	for i, s := 0, Next(g.Dealer); i < 4; i, s = i+1, Next(s) {
		for _, slot := range h.BurnSlots[s] {
			h.Discarded = append(h.Discarded, h.Hands[s][slot])
			h.Hands[s][slot] = draw(h)
		}
		if burned := len(h.BurnSlots[s]); burned > 0 {
			evs = append(evs, Event{Kind: EvFoursBurned, Visibility: visAll, Seat: seatPtr(s), Count: burned})
		}
	}
	evs = append(evs, revealVira(g)...)
	return append(evs, beginAfterVira(g)...)
}

func revealVira(g *GameState) []Event {
	h := g.Hand
	vira := draw(h)
	h.Vira = &vira
	h.Manilha = ManilhaRank(vira)
	id := vira.ID()
	return []Event{{Kind: EvViraRevealed, Visibility: visAll, Card: &id}}
}

func beginAfterVira(g *GameState) []Event {
	if g.Hand.Kind == HandEleven {
		teamOnEleven := Team(0)
		if g.Score[1] == WinningScore-1 {
			teamOnEleven = 1
		}
		g.Hand.VotingTeam = teamPtr(teamOnEleven)
		g.Hand.Votes = [4]*VoteChoice{}
		g.Phase = PhaseElevenDecision
		return nil
	}
	startPlaying(g)
	return nil
}

func startPlaying(g *GameState) {
	g.Phase = PhasePlaying
	g.Hand.Turn = Next(g.Dealer)
	g.Hand.RoundStarter = g.Hand.Turn
}
