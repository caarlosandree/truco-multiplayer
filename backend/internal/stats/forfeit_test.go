package stats_test

import (
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/enginetest"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
)

func TestForfeitedMatchRebuildsAndDerives(t *testing.T) {
	rec, _ := enginetest.RandomRecord(5)
	rec.Actions = rec.Actions[:20]
	rec.Actions = append(rec.Actions, replay.LoggedAction{Ply: 20,
		Action: engine.Action{Kind: engine.ActSysForfeit, Team: 1}, Source: replay.SourceSystem})
	final, err := replay.Rebuild(rec)
	if err != nil {
		t.Fatal(err)
	}
	if final.Phase != engine.PhaseGameOver || *final.Winner != 0 {
		t.Fatalf("fase %s vencedor %v", final.Phase, final.Winner)
	}
	perSeat, err := stats.Derive(rec)
	if err != nil {
		t.Fatal(err)
	}
	for seat, p := range perSeat {
		if p.Won != (engine.TeamOf(engine.Seat(seat)) == 0) || p.PointsFor+p.PointsAgainst == 0 {
			t.Fatalf("assento %d: %+v", seat, p)
		}
	}
}
