package bot

import (
	"context"
	"math/rand/v2"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// Adversários sintéticos para calibrar e conferir o Strong. Todos jogam as
// cartas como o Simple e diferem só nas decisões de aposta. O Simple não muda.

// acceptAll aceita todo aumento e joga toda mão de 11.
type acceptAll struct{ Simple }

func (acceptAll) Decide(ctx context.Context, in Input, rng *rand.Rand) Decision {
	switch in.View.Phase {
	case engine.PhaseAwaitingRaiseResponse:
		if a, ok := findVote(in.Legal, engine.VoteAccept); ok {
			return Decision{Action: a}
		}
	case engine.PhaseElevenDecision:
		if a, ok := findVote(in.Legal, engine.VotePlay); ok {
			return Decision{Action: a}
		}
	}
	return Simple{}.Decide(ctx, in, rng)
}

// runMore só aceita aumento com manilha (o Simple aceita também com duas altas).
type runMore struct{ Simple }

func (runMore) Decide(ctx context.Context, in Input, rng *rand.Rand) Decision {
	if in.View.Phase == engine.PhaseAwaitingRaiseResponse {
		want := engine.VoteRun
		if hasManilha(in.View) {
			want = engine.VoteAccept
		}
		if a, ok := findVote(in.Legal, want); ok {
			return Decision{Action: a}
		}
	}
	return Simple{}.Decide(ctx, in, rng)
}

// raiseHappy pede aumento sempre que pode, re-aumenta com manilha e aceita com
// qualquer carta alta (A, 2 ou 3) ou manilha.
type raiseHappy struct{ Simple }

func (raiseHappy) Decide(ctx context.Context, in Input, rng *rand.Rand) Decision {
	v := in.View
	switch v.Phase {
	case engine.PhasePlaying:
		if a, ok := findLegal(in.Legal, func(a engine.Action) bool { return a.Kind == engine.ActRequestRaise }); ok && v.HandKind == engine.HandNormal {
			return Decision{Action: a}
		}
	case engine.PhaseAwaitingRaiseResponse:
		if hasManilha(v) {
			if a, ok := findVote(in.Legal, engine.VoteRaise); ok {
				return Decision{Action: a}
			}
		}
		want := engine.VoteRun
		if hasManilha(v) || highCards(v) >= 1 {
			want = engine.VoteAccept
		}
		if a, ok := findVote(in.Legal, want); ok {
			return Decision{Action: a}
		}
	}
	return Simple{}.Decide(ctx, in, rng)
}

var syntheticOpponents = []struct {
	name string
	pol  Policy
}{
	{"simple", Simple{}},
	{"aceita-tudo", acceptAll{}},
	{"corre-mais", runMore{}},
	{"aumenta-muito", raiseHappy{}},
}

// O Strong não fica perto de 50% contra nenhum adversário sintético.
func TestStrongBeatsSyntheticOpponents(t *testing.T) {
	st := DefaultStrong()
	st.Samples = 40
	for _, o := range syntheticOpponents {
		w := strongVsOpponent(st, o.pol, 1, 120)
		t.Logf("%s: %d/120", o.name, w)
		if w < 72 { // 60% de 120
			t.Errorf("contra %s: %d/120 vitórias", o.name, w)
		}
	}
}

// TestBluffSweep varre bluffProb e bluffTolerance (BOT_SWEEP=jogos por célula).
func TestBluffSweep(t *testing.T) {
	games, _ := strconv.Atoi(os.Getenv("BOT_SWEEP"))
	if games == 0 {
		t.Skip("defina BOT_SWEEP=<jogos por adversário>")
	}
	oldP, oldT := bluffProb, bluffTolerance
	defer func() { bluffProb, bluffTolerance = oldP, oldT }()
	st := DefaultStrong()
	for _, tol := range []float64{0.03, 0.06, 0.10} {
		for _, p := range []float64{0, 0.15, 0.3, 0.6} {
			bluffProb, bluffTolerance = p, tol
			if p == 0 && tol != 0.03 {
				continue
			}
			line, sum := "", 0.0
			t0 := time.Now()
			for _, o := range syntheticOpponents {
				w := strongVsOpponent(st, o.pol, 1000, games)
				pct := 100 * float64(w) / float64(games)
				sum += pct
				line += " " + o.name + "=" + strconv.FormatFloat(pct, 'f', 1, 64)
			}
			t.Logf("tol=%.2f prob=%.2f média=%.1f |%s (%v)", tol, p, sum/float64(len(syntheticOpponents)), line, time.Since(t0).Round(time.Second))
		}
	}
}
