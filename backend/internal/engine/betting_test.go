package engine

import "testing"

func act(kind ActionKind, seat Seat) Action { return Action{Kind: kind, Seat: seat} }
func voteAs(seat Seat, choice VoteChoice) Action {
	return Action{Kind: ActVote, Seat: seat, Choice: choice}
}

func TestRaiseAcceptedReturnsTurnToRequester(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	g, _ = mustApply(t, g, act(ActRequestRaise, 1))
	if g.Phase != PhaseAwaitingRaiseResponse || *g.Hand.VotingTeam != 0 {
		t.Fatalf("fase=%s", g.Phase)
	}
	if *g.Hand.Pending != (PendingRaise{Requester: 1, FromTeam: 1, PrevStake: 1, ToStake: 3}) {
		t.Fatalf("pending = %+v", *g.Hand.Pending)
	}
	_, _, err := Apply(g, voteAs(1, VoteAccept))
	wantCode(t, err, ErrNotVotingTeam)
	g, _ = mustApply(t, g, voteAs(0, VoteAccept))
	if g.Phase != PhaseAwaitingRaiseResponse {
		t.Fatal("aguarda o voto do parceiro")
	}
	_, _, err = Apply(g, voteAs(0, VoteRun))
	wantCode(t, err, ErrAlreadyVoted)
	g, _ = mustApply(t, g, voteAs(2, VoteRun)) // mais ousado prevalece: aceitar
	if g.Phase != PhasePlaying || g.Hand.Stake != 3 || g.Hand.Turn != 1 || *g.Hand.LastRaiser != 1 {
		t.Fatalf("fase=%s stake=%d vez=%d", g.Phase, g.Hand.Stake, g.Hand.Turn)
	}
	_, _, err = Apply(g, act(ActRequestRaise, 1))
	wantCode(t, err, ErrIllegalRaise) // mesma dupla não aumenta duas vezes seguidas
}

func TestRaiseRunAwardsPreviousStake(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	g, _ = mustApply(t, g, act(ActRequestRaise, 1))
	g, _ = mustApply(t, g, voteAs(0, VoteRun))
	g, _ = mustApply(t, g, voteAs(2, VoteRun))
	if g.Score != [2]int{0, 1} || g.Phase != PhaseHandEnd {
		t.Fatalf("placar=%v fase=%s", g.Score, g.Phase)
	}
}

func TestCounterRaiseFlipsVotingTeam(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	g, _ = mustApply(t, g, act(ActRequestRaise, 1))
	g, _ = mustApply(t, g, voteAs(0, VoteRaise))
	g, _ = mustApply(t, g, voteAs(2, VoteRun))
	if *g.Hand.Pending != (PendingRaise{Requester: 1, FromTeam: 0, PrevStake: 3, ToStake: 6}) || *g.Hand.VotingTeam != 1 {
		t.Fatalf("pending = %+v", *g.Hand.Pending)
	}
	g, _ = mustApply(t, g, voteAs(1, VoteAccept))
	g, _ = mustApply(t, g, voteAs(3, VoteAccept))
	if g.Hand.Stake != 6 || g.Hand.Turn != 1 || *g.Hand.LastRaiser != 0 || g.Phase != PhasePlaying {
		t.Fatalf("stake=%d vez=%d", g.Hand.Stake, g.Hand.Turn)
	}
	g, _ = mustApply(t, g, act(ActRequestRaise, 1)) // B pode pedir 9: o último aumento foi da A
	if g.Hand.Pending.ToStake != 9 {
		t.Fatalf("toStake = %d", g.Hand.Pending.ToStake)
	}
}

func TestRunAfterCounterRaiseAwardsAcceptedStake(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	g, _ = mustApply(t, g, act(ActRequestRaise, 1))
	g, _ = mustApply(t, g, voteAs(0, VoteRaise))
	g, _ = mustApply(t, g, voteAs(2, VoteRaise))
	g, _ = mustApply(t, g, voteAs(1, VoteRun))
	g, _ = mustApply(t, g, voteAs(3, VoteRun))
	if g.Score != [2]int{3, 0} {
		t.Fatalf("placar = %v, quer A=3", g.Score)
	}
}

func TestRaiseCeilingAtTwelve(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	g.Hand.Stake = 12
	_, _, err := Apply(g, act(ActRequestRaise, 1))
	wantCode(t, err, ErrIllegalRaise)

	g.Hand.Stake = 9
	g, _ = mustApply(t, g, act(ActRequestRaise, 1))
	_, _, err = Apply(g, voteAs(0, VoteRaise))
	wantCode(t, err, ErrInvalidChoice)
}

func TestRaiseOnlyOnOwnTurn(t *testing.T) {
	g := playingState(fixtureBWinsTwice(), HandNormal)
	_, _, err := Apply(g, act(ActRequestRaise, 2))
	wantCode(t, err, ErrNotYourTurn)
}

func TestRaiseInElevenOrIronHandLosesTheGame(t *testing.T) {
	for _, kind := range []HandKind{HandEleven, HandIron} {
		g := playingState(fixtureBWinsTwice(), kind)
		g.Score = [2]int{11, 4}
		g, _ = mustApply(t, g, act(ActRequestRaise, 1))
		if g.Phase != PhaseGameOver || *g.Winner != 0 || g.Score[0] != 12 {
			t.Fatalf("%s: fase=%s placar=%v", kind, g.Phase, g.Score)
		}
	}
}

func TestElevenDecisionPlayOrRun(t *testing.T) {
	start := newTestGame(11)
	start.Score = [2]int{11, 3}
	start, _ = mustApply(t, start, Action{Kind: ActSysStartHand})
	start = dealWithoutBurning(t, start)

	_, _, err := Apply(start, voteAs(1, VotePlay))
	wantCode(t, err, ErrNotVotingTeam)
	_, _, err = Apply(start, voteAs(0, VoteAccept))
	wantCode(t, err, ErrInvalidChoice)

	played, _ := mustApply(t, start, voteAs(0, VotePlay))
	played, _ = mustApply(t, played, voteAs(2, VoteRun))
	if played.Phase != PhasePlaying || played.Hand.Stake != 3 {
		t.Fatalf("jogar prevalece: fase=%s stake=%d", played.Phase, played.Hand.Stake)
	}

	ran, _ := mustApply(t, start, voteAs(0, VoteRun))
	ran, _ = mustApply(t, ran, voteAs(2, VoteRun))
	if ran.Score != [2]int{11, 4} || ran.Phase != PhaseHandEnd {
		t.Fatalf("correr da mão de 11: placar=%v", ran.Score)
	}
}
