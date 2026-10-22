package replay

import (
	"errors"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

var testSeats = [4]PlayerSeat{{UserID: "u0", DisplayName: "Ana"}, {UserID: "u1", DisplayName: "Bia"}, {UserID: "u2", DisplayName: "Caio"}, {UserID: "u3", DisplayName: "Duda"}}

func newRecord(actions ...LoggedAction) Record {
	return Record{ID: "m1", Seed: 42, FirstDealer: 0, EngineVersion: engine.RulesVersion,
		Status: StatusInProgress, Players: testSeats, Actions: actions}
}

func start(ply int) LoggedAction {
	return LoggedAction{Ply: ply, Action: engine.Action{Kind: engine.ActSysStartHand}, Source: SourceSystem}
}

func TestRebuildAppliesActionsInOrder(t *testing.T) {
	g, err := Rebuild(newRecord(start(0)))
	if err != nil {
		t.Fatal(err)
	}
	if g.Phase != engine.PhaseCutChoice {
		t.Fatalf("fase = %s, quer cut_choice", g.Phase)
	}
	if g.Players[2].ID != "u2" || g.Players[2].Name != "Caio" {
		t.Fatalf("jogador 2 = %+v", g.Players[2])
	}
}

func TestRebuildEmptyLogIsNewGame(t *testing.T) {
	g, err := Rebuild(newRecord())
	if err != nil || g.Phase != engine.PhaseWaiting || g.Seed != 42 {
		t.Fatalf("g=%+v err=%v", g.Phase, err)
	}
}

func TestRebuildRejectsIncompatibleVersion(t *testing.T) {
	rec := newRecord(start(0))
	rec.EngineVersion = engine.RulesVersion + 1
	if _, err := Rebuild(rec); !errors.Is(err, ErrIncompatibleVersion) {
		t.Fatalf("err = %v", err)
	}
}

func TestRebuildReportsIllegalActionPly(t *testing.T) {
	illegal := LoggedAction{Ply: 1, Action: engine.Action{Kind: engine.ActPlayCard, Seat: 0, Slot: 0}, Source: SourcePlayer}
	_, err := Rebuild(newRecord(start(0), illegal))
	var corrupt *CorruptLogError
	if !errors.As(err, &corrupt) || corrupt.Ply != 1 {
		t.Fatalf("err = %v", err)
	}
	var rule *engine.RuleError
	if !errors.As(err, &rule) {
		t.Fatalf("CorruptLogError deve embrulhar o RuleError: %v", err)
	}
}

func TestRebuildRejectsPlyGap(t *testing.T) {
	_, err := Rebuild(newRecord(start(0), start(2)))
	var corrupt *CorruptLogError
	if !errors.As(err, &corrupt) || corrupt.Ply != 1 {
		t.Fatalf("err = %v", err)
	}
}

func TestRebuildRejectsLoggedSysTimeout(t *testing.T) {
	timeout := LoggedAction{Ply: 1, Action: engine.Action{Kind: engine.ActSysTimeout, Seat: 3}, Source: SourceTimeout}
	_, err := Rebuild(newRecord(start(0), timeout))
	var corrupt *CorruptLogError
	if !errors.As(err, &corrupt) || corrupt.Ply != 1 {
		t.Fatalf("sys_timeout nunca é gravado; err = %v", err)
	}
}

func TestRebuildRejectsInvalidDealer(t *testing.T) {
	rec := newRecord()
	rec.FirstDealer = 4
	var corrupt *CorruptLogError
	if _, err := Rebuild(rec); !errors.As(err, &corrupt) {
		t.Fatalf("err = %v", err)
	}
}

func TestWalkReportsBeforeAfterAndEvents(t *testing.T) {
	var steps []Step
	_, err := Walk(newRecord(start(0)), func(s Step) error {
		steps = append(steps, s)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 {
		t.Fatalf("%d passos", len(steps))
	}
	s := steps[0]
	if s.Before.Phase != engine.PhaseWaiting || s.After.Phase != engine.PhaseCutChoice || len(s.Events) == 0 {
		t.Fatalf("passo = before %s, after %s, %d eventos", s.Before.Phase, s.After.Phase, len(s.Events))
	}
}

func TestWalkStopsOnCallbackError(t *testing.T) {
	stop := errors.New("pare")
	_, err := Walk(newRecord(start(0)), func(Step) error { return stop })
	if !errors.Is(err, stop) {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildFramesHasOneViewPerSeat(t *testing.T) {
	frames, err := BuildFrames(newRecord(start(0)))
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 1 || frames[0].Ply != 0 || frames[0].Source != SourceSystem {
		t.Fatalf("frames = %+v", frames)
	}
	for seat, view := range frames[0].Views {
		if view.Seat != engine.Seat(seat) || view.Phase != engine.PhaseCutChoice {
			t.Fatalf("view %d = seat %d fase %s", seat, view.Seat, view.Phase)
		}
	}
}

func TestSourceValid(t *testing.T) {
	for _, s := range []Source{SourcePlayer, SourceTimeout, SourceSystem, SourceBot} {
		if !s.Valid() {
			t.Fatalf("%q deveria ser válido", s)
		}
	}
	if Source("alien").Valid() {
		t.Fatal("origem desconhecida não é válida")
	}
}
