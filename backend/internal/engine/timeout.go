package engine

// DefaultAction é o que acontece quando o tempo do jogador acaba.
// ok=false significa que o assento não tem decisão pendente.
func DefaultAction(g GameState, seat Seat) (Action, bool) {
	h := g.Hand
	switch g.Phase {
	case PhaseCutChoice:
		if seat == g.Cutter() {
			return Action{Kind: ActChooseCard, Seat: seat, Keep: 0}, true
		}
	case PhaseDealerChoice:
		if seat == g.Dealer {
			return Action{Kind: ActChooseCard, Seat: seat, Keep: 0}, true
		}
	case PhaseBurnFours:
		if !h.BurnDecided[seat] {
			return Action{Kind: ActPassBurn, Seat: seat}, true
		}
	case PhaseElevenDecision, PhaseAwaitingRaiseResponse:
		if TeamOf(seat) == *h.VotingTeam && h.Votes[seat] == nil {
			return Action{Kind: ActVote, Seat: seat, Choice: VoteRun}, true
		}
	case PhasePlaying:
		if seat != h.Turn {
			break
		}
		for slot := range h.Hands[seat] {
			if !h.Played[seat][slot] {
				return Action{Kind: ActPlayCard, Seat: seat, Slot: slot, Covered: h.Kind != HandIron}, true
			}
		}
	}
	return Action{}, false
}
