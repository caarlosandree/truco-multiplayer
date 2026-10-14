package engine

import "slices"

var stakeLadder = []int{1, 3, 6, 9, 12}

func nextStake(stake int) (int, bool) {
	i := slices.Index(stakeLadder, stake)
	if i < 0 || i == len(stakeLadder)-1 {
		return 0, false
	}
	return stakeLadder[i+1], true
}

func requestRaise(g *GameState, a Action) ([]Event, error) {
	if g.Phase != PhasePlaying {
		return nil, ruleErr(ErrInvalidPhase, "não é hora de pedir truco")
	}
	h := g.Hand
	if a.Seat != h.Turn {
		return nil, ruleErr(ErrNotYourTurn, "só se pede truco na própria vez")
	}
	team := TeamOf(a.Seat)
	if h.Kind == HandEleven || h.Kind == HandIron {
		opponent := Other(team)
		g.Score[opponent] = WinningScore
		evs := []Event{{Kind: EvRaisePunished, Visibility: visAll, Seat: seatPtr(a.Seat), Team: teamPtr(team)}}
		return append(evs, finishGame(g, opponent)...), nil
	}
	if h.LastRaiser != nil && *h.LastRaiser == team {
		return nil, ruleErr(ErrIllegalRaise, "sua dupla aumentou por último")
	}
	to, ok := nextStake(h.Stake)
	if !ok {
		return nil, ruleErr(ErrIllegalRaise, "aposta já está no máximo")
	}
	h.Pending = &PendingRaise{Requester: a.Seat, FromTeam: team, PrevStake: h.Stake, ToStake: to}
	openVote(g, Other(team), PhaseAwaitingRaiseResponse)
	return []Event{{Kind: EvRaiseRequested, Visibility: visAll, Seat: seatPtr(a.Seat), Team: teamPtr(team), Stake: to}}, nil
}

func openVote(g *GameState, team Team, phase Phase) {
	g.Hand.VotingTeam = teamPtr(team)
	g.Hand.Votes = [4]*VoteChoice{}
	g.Phase = phase
}

func vote(g *GameState, a Action) ([]Event, error) {
	if g.Phase != PhaseAwaitingRaiseResponse && g.Phase != PhaseElevenDecision {
		return nil, ruleErr(ErrInvalidPhase, "não há votação aberta")
	}
	h := g.Hand
	if TeamOf(a.Seat) != *h.VotingTeam {
		return nil, ruleErr(ErrNotVotingTeam, "sua dupla não está votando")
	}
	if h.Votes[a.Seat] != nil {
		return nil, ruleErr(ErrAlreadyVoted, "voto já registrado")
	}
	if !validChoice(g, a.Choice) {
		return nil, ruleErr(ErrInvalidChoice, "opção de voto inválida: "+string(a.Choice))
	}
	choice := a.Choice
	h.Votes[a.Seat] = &choice
	evs := []Event{{Kind: EvVoteCast, Visibility: visTeam(*h.VotingTeam), Seat: seatPtr(a.Seat), Choice: choice}}
	partnerVote := h.Votes[Partner(a.Seat)]
	if partnerVote == nil {
		return evs, nil
	}
	final := boldest(choice, *partnerVote)
	if g.Phase == PhaseElevenDecision {
		return append(evs, resolveEleven(g, final)...), nil
	}
	return append(evs, resolveRaise(g, final)...), nil
}

func validChoice(g *GameState, choice VoteChoice) bool {
	if g.Phase == PhaseElevenDecision {
		return choice == VotePlay || choice == VoteRun
	}
	switch choice {
	case VoteAccept, VoteRun:
		return true
	case VoteRaise:
		_, canRaise := nextStake(g.Hand.Pending.ToStake)
		return canRaise
	}
	return false
}

// boldness ordena os votos: na divergência prevalece o mais ousado.
var boldness = map[VoteChoice]int{VoteRun: 0, VoteAccept: 1, VotePlay: 2, VoteRaise: 2}

func boldest(a, b VoteChoice) VoteChoice {
	if boldness[b] > boldness[a] {
		return b
	}
	return a
}

func resolveRaise(g *GameState, choice VoteChoice) []Event {
	h := g.Hand
	pending := *h.Pending
	voting := *h.VotingTeam
	switch choice {
	case VoteAccept:
		h.Stake = pending.ToStake
		h.LastRaiser = teamPtr(pending.FromTeam)
		h.Pending, h.VotingTeam = nil, nil
		h.Votes = [4]*VoteChoice{}
		h.Turn = pending.Requester
		g.Phase = PhasePlaying
		return []Event{{Kind: EvRaiseResolved, Visibility: visAll, Team: teamPtr(voting), Choice: VoteAccept, Stake: h.Stake}}
	case VoteRun:
		evs := []Event{{Kind: EvRaiseResolved, Visibility: visAll, Team: teamPtr(voting), Choice: VoteRun, Stake: pending.PrevStake}}
		return append(evs, endHand(g, teamPtr(pending.FromTeam), pending.PrevStake)...)
	}
	to, _ := nextStake(pending.ToStake) // validChoice já garantiu o próximo degrau
	h.Pending = &PendingRaise{Requester: pending.Requester, FromTeam: voting, PrevStake: pending.ToStake, ToStake: to}
	h.LastRaiser = teamPtr(voting)
	openVote(g, Other(voting), PhaseAwaitingRaiseResponse)
	return []Event{{Kind: EvRaiseRequested, Visibility: visAll, Team: teamPtr(voting), Stake: to}}
}

func resolveEleven(g *GameState, choice VoteChoice) []Event {
	team := *g.Hand.VotingTeam
	decided := Event{Kind: EvElevenDecided, Visibility: visAll, Team: teamPtr(team), Choice: choice}
	g.Hand.VotingTeam = nil
	g.Hand.Votes = [4]*VoteChoice{}
	if choice == VoteRun {
		return append([]Event{decided}, endHand(g, teamPtr(Other(team)), 1)...)
	}
	startPlaying(g)
	return []Event{decided}
}
