package engine

// LegalActions enumera as ações de jogador que Apply aceita para o assento no
// estado atual. Nunca devolve ações sys_*; a ordem é determinística (por fase,
// depois por slot/escolha em ordem crescente). Sem decisão pendente devolve nil.
func LegalActions(g GameState, seat Seat) []Action {
	if seat > 3 || g.Hand == nil {
		return nil
	}
	// Quem tem alguma decisão pendente sempre tem DefaultAction (mesma lógica por fase).
	if _, ok := DefaultAction(g, seat); !ok {
		return nil
	}
	var out []Action
	for _, a := range candidatesFor(g, seat) {
		if _, _, err := Apply(g, a); err == nil {
			out = append(out, a)
		}
	}
	return out
}

// candidatesFor gera o superconjunto de ações da fase; Apply filtra as ilegais.
func candidatesFor(g GameState, seat Seat) []Action {
	h := g.Hand
	var out []Action
	switch g.Phase {
	case PhaseCutChoice, PhaseDealerChoice:
		out = append(out,
			Action{Kind: ActChooseCard, Seat: seat, Keep: 0},
			Action{Kind: ActChooseCard, Seat: seat, Keep: 1})
	case PhaseBurnFours:
		out = append(out, Action{Kind: ActPassBurn, Seat: seat})
		var fours []int
		for slot, card := range h.Hands[seat] {
			if card.Rank == 4 {
				fours = append(fours, slot)
			}
		}
		// Todos os subconjuntos não vazios de quatros, em ordem de máscara.
		for mask := 1; mask < 1<<len(fours); mask++ {
			var slots []int
			for i, slot := range fours {
				if mask&(1<<i) != 0 {
					slots = append(slots, slot)
				}
			}
			out = append(out, Action{Kind: ActBurn, Seat: seat, Slots: slots})
		}
	case PhaseElevenDecision, PhaseAwaitingRaiseResponse:
		for _, choice := range []VoteChoice{VoteAccept, VoteRun, VoteRaise, VotePlay} {
			out = append(out, Action{Kind: ActVote, Seat: seat, Choice: choice})
		}
	case PhasePlaying:
		for slot := range h.Hands[seat] {
			out = append(out, Action{Kind: ActPlayCard, Seat: seat, Slot: slot})
			out = append(out, Action{Kind: ActPlayCard, Seat: seat, Slot: slot, Covered: true})
		}
		out = append(out, Action{Kind: ActRequestRaise, Seat: seat})
	}
	return out
}
