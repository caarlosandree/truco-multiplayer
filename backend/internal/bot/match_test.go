package bot

import (
	"context"
	"math/rand/v2"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

func TestPlayMatchIsDeterministicAndFinishes(t *testing.T) {
	st := Strong{Samples: 8}
	policies := [4]Policy{st, Simple{}, st, Simple{}}
	for seed := uint64(1); seed <= 12; seed++ {
		a, b := PlayMatch(policies, seed), PlayMatch(policies, seed)
		if a != b {
			t.Fatalf("seed %d: vencedores diferentes (%d e %d)", seed, a, b)
		}
	}
	// Todas as duplas ganham alguma partida de Simple contra Simple: a partida termina de verdade.
	wins := [2]int{}
	simple := [4]Policy{Simple{}, Simple{}, Simple{}, Simple{}}
	for seed := uint64(1); seed <= 40; seed++ {
		wins[PlayMatch(simple, seed)]++
	}
	if wins[0] == 0 || wins[1] == 0 {
		t.Fatalf("vitórias Simple x Simple: %v", wins)
	}
}

func mustPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s: esperava panic", name)
		}
	}()
	f()
}

// A régua do benchmark falha alto: nada de vencedor "por placar" quando algo dá errado.
type badPolicy struct{ act engine.Action }

func (b badPolicy) Decide(context.Context, Input, *rand.Rand) Decision {
	return Decision{Action: b.act}
}

func TestPlayMatchFailsLoudly(t *testing.T) {
	simple := [4]Policy{Simple{}, Simple{}, Simple{}, Simple{}}
	nilSeat := simple
	nilSeat[1] = nil
	mustPanic(t, "política nula", func() { PlayMatch(nilSeat, 1) })
	illegal := simple
	illegal[0], illegal[1], illegal[2], illegal[3] = badPolicy{engine.Action{Kind: engine.ActPlayCard, Slot: 9}}, badPolicy{engine.Action{Kind: engine.ActPlayCard, Slot: 9}}, badPolicy{engine.Action{Kind: engine.ActPlayCard, Slot: 9}}, badPolicy{engine.Action{Kind: engine.ActPlayCard, Slot: 9}}
	mustPanic(t, "ação rejeitada pelo engine", func() { PlayMatch(illegal, 1) })
}
