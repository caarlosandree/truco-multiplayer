// Package stats deriva estatísticas de partidas a partir do log e consulta o ranking.
package stats

import (
	"errors"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

type PlayerMatchStats struct {
	UserID          string
	Team            engine.Team
	Won             bool
	PointsFor       int
	PointsAgainst   int
	HandsPlayed     int
	HandsWon        int
	RaisesCalled    int
	RaisesFolded    int
	RaisesWonByFold int
	ElevenHands     int
	ElevenRuns      int
	ElevenWon       int
	IronHands       int
	IronWon         int
	Punished        int
	Timeouts        int
}

var ErrNotFinished = errors.New("stats: partida não terminou")

// Derive percorre o log e conta, por assento, as estatísticas da partida.
func Derive(rec replay.Record) ([4]PlayerMatchStats, error) {
	var acc accumulator
	final, err := replay.Walk(rec, func(s replay.Step) error {
		acc.observe(s)
		return nil
	})
	if err != nil {
		return [4]PlayerMatchStats{}, err
	}
	if final.Phase != engine.PhaseGameOver || final.Winner == nil {
		return [4]PlayerMatchStats{}, ErrNotFinished
	}
	return acc.result(rec, final), nil
}

type seatCounters struct {
	raisesCalled, punished, timeouts int
}

type teamCounters struct {
	handsWon, raisesFolded, raisesWonByFold int
	elevenHands, elevenRuns, elevenWon      int
	ironHands, ironWon                      int
}

// accumulator guarda a mão corrente para atribuir resultados de Mão de 11/Ferro.
type accumulator struct {
	seat        [4]seatCounters
	team        [2]teamCounters
	handsPlayed int
	handKind    engine.HandKind
	elevenTeam  *engine.Team
}

func (a *accumulator) observe(s replay.Step) {
	action := s.Logged.Action
	if s.Logged.Source == replay.SourceTimeout {
		a.seat[action.Seat].timeouts++
	}
	isRaise := action.Kind == engine.ActRequestRaise || (action.Kind == engine.ActVote && action.Choice == engine.VoteRaise)
	if isRaise {
		a.seat[action.Seat].raisesCalled++
	}
	if action.Kind == engine.ActSysStartHand && s.After.Hand != nil {
		a.startHand(s.After)
	}
	for _, e := range s.Events {
		a.event(e)
	}
}

func (a *accumulator) startHand(g engine.GameState) {
	a.handKind, a.elevenTeam = g.Hand.Kind, nil
	switch g.Hand.Kind {
	case engine.HandEleven:
		for t := engine.Team(0); t < 2; t++ {
			if g.Score[t] == engine.WinningScore-1 {
				team := t
				a.elevenTeam = &team
				a.team[t].elevenHands++
			}
		}
	case engine.HandIron:
		a.team[0].ironHands++
		a.team[1].ironHands++
	}
}

func (a *accumulator) event(e engine.Event) {
	switch e.Kind {
	case engine.EvHandEnded:
		a.handsPlayed++
		if e.Team == nil {
			return
		}
		winner := *e.Team
		a.team[winner].handsWon++
		if a.handKind == engine.HandEleven && a.elevenTeam != nil && *a.elevenTeam == winner {
			a.team[winner].elevenWon++
		}
		if a.handKind == engine.HandIron {
			a.team[winner].ironWon++
		}
	case engine.EvRaiseResolved:
		if e.Choice == engine.VoteRun && e.Team != nil {
			a.team[*e.Team].raisesFolded++
			a.team[engine.Other(*e.Team)].raisesWonByFold++
		}
	case engine.EvElevenDecided:
		if e.Choice == engine.VoteRun && e.Team != nil {
			a.team[*e.Team].elevenRuns++
		}
	case engine.EvRaisePunished:
		if e.Seat != nil {
			a.seat[*e.Seat].punished++
		}
	}
}

func (a *accumulator) result(rec replay.Record, final engine.GameState) [4]PlayerMatchStats {
	var out [4]PlayerMatchStats
	for seat := range 4 {
		team := engine.TeamOf(engine.Seat(seat))
		tc, sc := a.team[team], a.seat[seat]
		out[seat] = PlayerMatchStats{
			UserID: rec.Players[seat].UserID, Team: team, Won: *final.Winner == team,
			PointsFor: final.Score[team], PointsAgainst: final.Score[engine.Other(team)],
			HandsPlayed: a.handsPlayed, HandsWon: tc.handsWon,
			RaisesCalled: sc.raisesCalled, RaisesFolded: tc.raisesFolded, RaisesWonByFold: tc.raisesWonByFold,
			ElevenHands: tc.elevenHands, ElevenRuns: tc.elevenRuns, ElevenWon: tc.elevenWon,
			IronHands: tc.ironHands, IronWon: tc.ironWon,
			Punished: sc.punished, Timeouts: sc.timeouts,
		}
	}
	return out
}
