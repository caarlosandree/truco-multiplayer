package bot

import (
	"context"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// inputFor monta o Input de um assento a partir do estado real. É helper de
// TESTE: nenhuma função exportada do pacote recebe GameState.
func inputFor(g engine.GameState, seat engine.Seat, evs []engine.Event) Input {
	return Input{
		View:   engine.ViewFor(g, seat),
		Events: engine.FilterEvents(evs, seat),
		Legal:  engine.LegalActions(g, seat),
	}
}

func actionIn(legal []engine.Action, a engine.Action) bool {
	return slices.ContainsFunc(legal, func(l engine.Action) bool { return sameAction(l, a) })
}

func TestPoliciesAlwaysReturnLegalAction(t *testing.T) {
	policies := map[string]Policy{"simple": Simple{}}
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
						in := inputFor(g, s, evs)
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
