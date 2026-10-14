package engine

import (
	"encoding/json"
	"testing"
)

func TestGameStateIsNotSerializable(t *testing.T) {
	g := newTestGame(1)
	if _, err := json.Marshal(g); err == nil {
		t.Fatal("json.Marshal(GameState) deveria falhar")
	}
	if _, err := json.Marshal(&g); err == nil {
		t.Fatal("json.Marshal(*GameState) deveria falhar")
	}
	g, _ = mustApply(t, g, Action{Kind: ActSysStartHand})
	if _, err := json.Marshal(ViewFor(g, 0)); err != nil {
		t.Fatalf("PlayerView deve continuar serializável: %v", err)
	}
}

func TestRulesVersionIsPositive(t *testing.T) {
	if RulesVersion < 1 {
		t.Fatalf("RulesVersion = %d", RulesVersion)
	}
}
