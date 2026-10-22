package room

import (
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

func TestObviousMoveDetection(t *testing.T) {
	play := func(slot int, covered bool) engine.Action {
		return engine.Action{Kind: engine.ActPlayCard, Slot: slot, Covered: covered}
	}
	raise := engine.Action{Kind: engine.ActRequestRaise}
	if !obviousMove([]engine.Action{play(2, false), play(2, true), raise}) {
		t.Fatal("última carta (um só slot, mais cobrir e aumento) é óbvia")
	}
	if obviousMove([]engine.Action{play(0, false), play(0, true), play(1, false), play(1, true), raise}) {
		t.Fatal("duas cartas não é óbvio")
	}
	if obviousMove([]engine.Action{{Kind: engine.ActVote, Choice: engine.VoteAccept}, {Kind: engine.ActVote, Choice: engine.VoteRun}}) {
		t.Fatal("votação não é óbvia")
	}
	if got := pauseFor(true, 2*time.Second); got != 500*time.Millisecond {
		t.Fatalf("óbvia: %v", got)
	}
	if got := pauseFor(false, 2*time.Second); got != 2*time.Second {
		t.Fatalf("normal: %v", got)
	}
}
