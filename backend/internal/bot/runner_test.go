package bot

import (
	"context"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// fakePolicy é uma política controlada pelo teste.
type fakePolicy struct {
	fn func(ctx context.Context, in Input) Decision
}

func (f fakePolicy) Decide(ctx context.Context, in Input, _ *rand.Rand) Decision {
	return f.fn(ctx, in)
}

func decisionOf(kind engine.ActionKind) Decision { return Decision{Action: engine.Action{Kind: kind}} }

func newTestRunner(strong, fallback Policy, max int) *Runner {
	return NewRunner(RunnerOptions{
		Strong: strong, Fallback: fallback, Budget: time.Second, MaxConcurrent: max,
		Seed: func() uint64 { return 1 },
	})
}

func TestRunnerDoesNotBlockCaller(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	r := newTestRunner(fakePolicy{func(ctx context.Context, _ Input) Decision {
		<-release
		return Decision{}
	}}, fakePolicy{func(context.Context, Input) Decision { return Decision{} }}, 1)
	start := time.Now()
	r.Think(Input{}, time.Now().Add(time.Minute), func(Decision) {})
	if d := time.Since(start); d > 20*time.Millisecond { // folga para -race e CI lento; a intenção é só não esperar a política
		t.Fatalf("Think bloqueou o chamador por %v", d)
	}
}

func TestRunnerRespectsMaxConcurrent(t *testing.T) {
	var cur, peak atomic.Int32
	r := newTestRunner(fakePolicy{func(ctx context.Context, _ Input) Decision {
		n := cur.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		cur.Add(-1)
		return Decision{}
	}}, fakePolicy{func(context.Context, Input) Decision { return Decision{} }}, 2)
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		r.Think(Input{}, time.Now().Add(time.Minute), func(Decision) { wg.Done() })
	}
	wg.Wait()
	if p := peak.Load(); p > 2 || p < 1 {
		t.Fatalf("pico de Decide simultâneos = %d, quero <= 2", p)
	}
}

func TestRunnerFallsBackWhenQueueExceedsDeadline(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	strong := fakePolicy{func(ctx context.Context, _ Input) Decision {
		close(started)
		<-release
		return decisionOf(engine.ActPlayCard)
	}}
	fb := fakePolicy{func(context.Context, Input) Decision { return decisionOf(engine.ActBurn) }}
	r := newTestRunner(strong, fb, 1)
	r.Think(Input{}, time.Now().Add(time.Minute), func(Decision) {})
	<-started
	got := make(chan Decision, 1)
	r.Think(Input{}, time.Now().Add(20*time.Millisecond), func(d Decision) { got <- d })
	select {
	case d := <-got:
		if d.Action.Kind != engine.ActBurn {
			t.Fatalf("esperava o fallback, veio %+v", d)
		}
	case <-time.After(time.Second):
		t.Fatal("done não foi chamado após o prazo")
	}
	close(release)
}

func TestRunnerCallsDoneOnce(t *testing.T) {
	fb := fakePolicy{func(context.Context, Input) Decision { return decisionOf(engine.ActBurn) }}
	cases := map[string]Policy{
		"normal": fakePolicy{func(context.Context, Input) Decision { return decisionOf(engine.ActPlayCard) }},
		"panico": fakePolicy{func(context.Context, Input) Decision { panic("boom") }},
	}
	for name, strong := range cases {
		t.Run(name, func(t *testing.T) {
			var n atomic.Int32
			got := make(chan Decision, 4)
			r := newTestRunner(strong, fb, 1)
			r.Think(Input{}, time.Now().Add(time.Minute), func(d Decision) { n.Add(1); got <- d })
			d := <-got
			time.Sleep(30 * time.Millisecond)
			if n.Load() != 1 {
				t.Fatalf("done chamado %d vezes", n.Load())
			}
			if name == "panico" && d.Action.Kind != engine.ActBurn {
				t.Fatalf("pânico deveria usar o fallback, veio %+v", d)
			}
		})
	}
}

func TestRunnerBudgetBoundsContext(t *testing.T) {
	got := make(chan time.Duration, 1)
	r := NewRunner(RunnerOptions{
		Strong: fakePolicy{func(ctx context.Context, _ Input) Decision {
			dl, ok := ctx.Deadline()
			if !ok {
				got <- -1
			} else {
				got <- time.Until(dl)
			}
			return Decision{}
		}},
		Fallback: fakePolicy{func(context.Context, Input) Decision { return Decision{} }},
		Budget:   time.Second, MaxConcurrent: 1, Seed: func() uint64 { return 1 },
	})
	r.Think(Input{}, time.Now().Add(100*time.Millisecond), func(Decision) {})
	if d := <-got; d <= 0 || d > 100*time.Millisecond {
		t.Fatalf("ctx deveria expirar em <= 100ms, expira em %v", d)
	}
}
