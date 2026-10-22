package bot

import (
	"context"
	"math/rand/v2"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

func TestPoliciesAlwaysReturnLegalAction(t *testing.T) {
	policies := map[string]Policy{"simple": Simple{}, "strong": Strong{Samples: 8}}
	players := [4]engine.Player{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}, {ID: "c", Name: "C"}, {ID: "d", Name: "D"}}
	for name, p := range policies {
		t.Run(name, func(t *testing.T) {
			for seed := uint64(1); seed <= 200; seed++ {
				rng := rand.New(rand.NewPCG(seed, 7))
				g := engine.NewGame(players, seed, engine.Seat(seed%4))
				var evs []engine.Event
				for step := 0; g.Phase != engine.PhaseGameOver; step++ {
					if step > 5000 {
						t.Fatalf("seed %d: partida não terminou", seed)
					}
					var a engine.Action
					acted := false
					for s := engine.Seat(0); s < 4; s++ {
						in := inputFromState(g, s, evs)
						if len(in.Legal) == 0 {
							continue
						}
						d := p.Decide(context.Background(), in, rng)
						if !actionIn(in.Legal, d.Action) {
							t.Fatalf("seed %d: decisão %+v fora de Legal %+v", seed, d.Action, in.Legal)
						}
						a, acted = d.Action, true
						break
					}
					if !acted {
						a = engine.Action{Kind: engine.ActSysStartHand}
					}
					next, newEvs, err := engine.Apply(g, a)
					if err != nil {
						t.Fatalf("seed %d: Apply(%+v): %v", seed, a, err)
					}
					for _, e := range newEvs {
						if e.Kind == engine.EvRaisePunished {
							t.Fatalf("seed %d: bot pediu aumento suicida (%+v)", seed, a)
						}
					}
					g, evs = next, append(evs, newEvs...)
				}
			}
		})
	}
}
