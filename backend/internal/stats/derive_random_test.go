package stats_test

import (
	"errors"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/enginetest"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
)

func TestDeriveInvariantsOnRandomGames(t *testing.T) {
	for seed := range uint64(200) {
		rec, final := enginetest.RandomRecord(seed)
		got, err := stats.Derive(rec)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		winners, timeouts, raises := 0, 0, 0
		for _, logged := range rec.Actions {
			if logged.Source == replay.SourceTimeout {
				timeouts++
			}
			a := logged.Action
			if a.Kind == engine.ActRequestRaise || (a.Kind == engine.ActVote && a.Choice == engine.VoteRaise) {
				raises++
			}
		}
		sumTimeouts, sumRaises := 0, 0
		for seat, s := range got {
			team := engine.TeamOf(engine.Seat(seat))
			if s.Team != team || s.UserID != rec.Players[seat].UserID {
				t.Fatalf("seed %d assento %d: %+v", seed, seat, s)
			}
			if s.PointsFor != final.Score[team] || s.PointsAgainst != final.Score[engine.Other(team)] {
				t.Fatalf("seed %d: pontos %d/%d, placar %v", seed, s.PointsFor, s.PointsAgainst, final.Score)
			}
			if s.Won {
				winners++
			}
			partner := got[(seat+2)%4]
			if s.HandsWon != partner.HandsWon || s.RaisesFolded != partner.RaisesFolded || s.ElevenWon != partner.ElevenWon || s.Won != partner.Won {
				t.Fatalf("seed %d: parceiros com contadores de dupla diferentes", seed)
			}
			sumTimeouts += s.Timeouts
			sumRaises += s.RaisesCalled
		}
		if winners != 2 {
			t.Fatalf("seed %d: %d vencedores (quer a dupla inteira)", seed, winners)
		}
		if got[0].HandsWon+got[1].HandsWon > got[0].HandsPlayed {
			t.Fatalf("seed %d: mãos vencidas > jogadas", seed)
		}
		if sumTimeouts != timeouts || sumRaises != raises {
			t.Fatalf("seed %d: timeouts %d/%d, aumentos %d/%d", seed, sumTimeouts, timeouts, sumRaises, raises)
		}
	}
}

func TestDeriveRejectsUnfinishedGame(t *testing.T) {
	rec, _ := enginetest.RandomRecord(3)
	rec.Actions = rec.Actions[:len(rec.Actions)/2]
	if _, err := stats.Derive(rec); !errors.Is(err, stats.ErrNotFinished) {
		t.Fatalf("err = %v", err)
	}
}

func TestDerivePropagatesCorruptLog(t *testing.T) {
	rec, _ := enginetest.RandomRecord(3)
	rec.Actions[1].Action = engine.Action{Kind: engine.ActRequestRaise, Seat: 0}
	var corrupt *replay.CorruptLogError
	if _, err := stats.Derive(rec); !errors.As(err, &corrupt) {
		t.Fatalf("err = %v", err)
	}
}
