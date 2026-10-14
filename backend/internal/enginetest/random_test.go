package enginetest

import (
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

func TestRandomRecordPlaysToGameOver(t *testing.T) {
	sawTimeout, sawSpecialHand := false, false
	for seed := range uint64(150) {
		rec, final := RandomRecord(seed)
		if final.Phase != engine.PhaseGameOver {
			t.Fatalf("seed %d: fase final %s", seed, final.Phase)
		}
		if len(rec.Actions) == 0 || rec.Actions[0].Action.Kind != engine.ActSysStartHand || rec.Actions[0].Source != replay.SourceSystem {
			t.Fatalf("seed %d: primeira ação deve ser sys_start_hand do sistema", seed)
		}
		for i, logged := range rec.Actions {
			if logged.Ply != i || !logged.Source.Valid() || logged.Action.Kind == engine.ActSysTimeout {
				t.Fatalf("seed %d ply %d: %+v", seed, i, logged)
			}
			if logged.Source == replay.SourceTimeout {
				sawTimeout = true
			}
		}
		_, _ = replay.Walk(rec, func(s replay.Step) error {
			if s.Logged.Action.Kind == engine.ActSysStartHand && s.After.Hand != nil && s.After.Hand.Kind != engine.HandNormal {
				sawSpecialHand = true
			}
			return nil
		})
	}
	if !sawTimeout {
		t.Fatal("nenhuma partida gerou timeout; o gerador deve exercitá-los")
	}
	if !sawSpecialHand {
		t.Fatal("nenhuma partida chegou a uma Mão de 11 ou de Ferro")
	}
}

func TestRandomRecordIsDeterministic(t *testing.T) {
	a, _ := RandomRecord(7)
	b, _ := RandomRecord(7)
	if len(a.Actions) != len(b.Actions) {
		t.Fatal("mesma seed deve gerar o mesmo log")
	}
}
