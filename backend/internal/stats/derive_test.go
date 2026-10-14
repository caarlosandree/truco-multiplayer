package stats

import (
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

func teamPtr(t engine.Team) *engine.Team { return &t }
func seatPtr(s engine.Seat) *engine.Seat { return &s }

func act(kind engine.ActionKind, seat engine.Seat) replay.LoggedAction {
	return replay.LoggedAction{Action: engine.Action{Kind: kind, Seat: seat}, Source: replay.SourcePlayer}
}

func startHandStep(kind engine.HandKind, score [2]int) replay.Step {
	return replay.Step{
		Logged: replay.LoggedAction{Action: engine.Action{Kind: engine.ActSysStartHand}, Source: replay.SourceSystem},
		After:  engine.GameState{Score: score, Hand: &engine.HandState{Kind: kind}},
	}
}

func eventStep(evs ...engine.Event) replay.Step {
	return replay.Step{Logged: act(engine.ActPlayCard, 0), Events: evs}
}

func TestAccumulatorElevenHandWonByTeamWithEleven(t *testing.T) {
	var a accumulator
	a.observe(startHandStep(engine.HandEleven, [2]int{11, 5}))
	a.observe(eventStep(engine.Event{Kind: engine.EvHandEnded, Team: teamPtr(0)}))
	if a.team[0].elevenHands != 1 || a.team[1].elevenHands != 0 {
		t.Fatalf("elevenHands = %d/%d", a.team[0].elevenHands, a.team[1].elevenHands)
	}
	if a.team[0].elevenWon != 1 || a.team[0].handsWon != 1 || a.handsPlayed != 1 {
		t.Fatalf("contadores = %+v, handsPlayed %d", a.team[0], a.handsPlayed)
	}
}

func TestAccumulatorElevenRunGivesHandToOpponent(t *testing.T) {
	var a accumulator
	a.observe(startHandStep(engine.HandEleven, [2]int{3, 11}))
	a.observe(eventStep(
		engine.Event{Kind: engine.EvElevenDecided, Team: teamPtr(1), Choice: engine.VoteRun},
		engine.Event{Kind: engine.EvHandEnded, Team: teamPtr(0), Stake: 1},
	))
	if a.team[1].elevenRuns != 1 || a.team[1].elevenWon != 0 || a.team[0].handsWon != 1 {
		t.Fatalf("time0 %+v time1 %+v", a.team[0], a.team[1])
	}
}

func TestAccumulatorIronHand(t *testing.T) {
	var a accumulator
	a.observe(startHandStep(engine.HandIron, [2]int{11, 11}))
	a.observe(eventStep(engine.Event{Kind: engine.EvHandEnded, Team: teamPtr(1)}))
	if a.team[0].ironHands != 1 || a.team[1].ironHands != 1 || a.team[1].ironWon != 1 || a.team[0].ironWon != 0 {
		t.Fatalf("time0 %+v time1 %+v", a.team[0], a.team[1])
	}
}

func TestAccumulatorRaiseFold(t *testing.T) {
	var a accumulator
	a.observe(eventStep(engine.Event{Kind: engine.EvRaiseResolved, Team: teamPtr(1), Choice: engine.VoteRun}))
	a.observe(eventStep(engine.Event{Kind: engine.EvRaiseResolved, Team: teamPtr(0), Choice: engine.VoteAccept}))
	if a.team[1].raisesFolded != 1 || a.team[0].raisesWonByFold != 1 || a.team[0].raisesFolded != 0 {
		t.Fatalf("time0 %+v time1 %+v", a.team[0], a.team[1])
	}
}

func TestAccumulatorIndividualCounters(t *testing.T) {
	var a accumulator
	a.observe(replay.Step{Logged: act(engine.ActRequestRaise, 1)})
	a.observe(replay.Step{Logged: replay.LoggedAction{Action: engine.Action{Kind: engine.ActVote, Seat: 2, Choice: engine.VoteRaise}, Source: replay.SourcePlayer}})
	a.observe(replay.Step{Logged: replay.LoggedAction{Action: engine.Action{Kind: engine.ActVote, Seat: 2, Choice: engine.VoteAccept}, Source: replay.SourcePlayer}})
	a.observe(replay.Step{Logged: replay.LoggedAction{Action: engine.Action{Kind: engine.ActPassBurn, Seat: 3}, Source: replay.SourceTimeout}})
	a.observe(eventStep(engine.Event{Kind: engine.EvRaisePunished, Seat: seatPtr(0), Team: teamPtr(0)}))
	if a.seat[1].raisesCalled != 1 || a.seat[2].raisesCalled != 1 || a.seat[3].timeouts != 1 || a.seat[0].punished != 1 {
		t.Fatalf("assentos = %+v", a.seat)
	}
}

func TestAccumulatorTiedHandCountsOnlyAsPlayed(t *testing.T) {
	var a accumulator
	a.observe(eventStep(engine.Event{Kind: engine.EvHandEnded}))
	if a.handsPlayed != 1 || a.team[0].handsWon != 0 || a.team[1].handsWon != 0 {
		t.Fatalf("handsPlayed %d, vitórias %d/%d", a.handsPlayed, a.team[0].handsWon, a.team[1].handsWon)
	}
}
