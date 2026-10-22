package engine

import (
	"math/rand/v2"
	"reflect"
	"testing"
)

// exhaustiveActions gera, sem olhar para a fase, tudo o que um assento poderia tentar.
func exhaustiveActions(g GameState, s Seat) []Action {
	out := []Action{
		{Kind: ActChooseCard, Seat: s, Keep: 0},
		{Kind: ActChooseCard, Seat: s, Keep: 1},
		{Kind: ActPassBurn, Seat: s},
		{Kind: ActRequestRaise, Seat: s},
	}
	for _, choice := range []VoteChoice{VoteAccept, VoteRun, VoteRaise, VotePlay} {
		out = append(out, Action{Kind: ActVote, Seat: s, Choice: choice})
	}
	for slot := range 3 {
		out = append(out,
			Action{Kind: ActPlayCard, Seat: s, Slot: slot},
			Action{Kind: ActPlayCard, Seat: s, Slot: slot, Covered: true})
	}
	if g.Hand != nil {
		for mask := 1; mask < 1<<len(g.Hand.Hands[s]); mask++ {
			var slots []int
			for i := range g.Hand.Hands[s] {
				if mask&(1<<i) != 0 {
					slots = append(slots, i)
				}
			}
			out = append(out, Action{Kind: ActBurn, Seat: s, Slots: slots})
		}
	}
	return out
}

func isSys(k ActionKind) bool {
	return k == ActSysStartHand || k == ActSysTimeout || k == ActSysForfeit
}

func TestLegalActionsMatchApply(t *testing.T) {
	kinds := map[HandKind]bool{}
	phases := map[Phase]bool{}
	for seed := uint64(1); seed <= 300; seed++ {
		rng := rand.New(rand.NewPCG(seed, 5))
		g := NewGame(testPlayers, seed, Seat(seed%4))
		for step := 0; g.Phase != PhaseGameOver; step++ {
			if step > 5000 {
				t.Fatalf("seed %d: partida não terminou", seed)
			}
			var legalAll []Action
			for s := Seat(0); s < 4; s++ {
				legal := LegalActions(g, s)
				if g.Hand != nil {
					kinds[g.Hand.Kind] = true
				}
				phases[g.Phase] = true
				inList := func(a Action) bool {
					for _, l := range legal {
						if reflect.DeepEqual(l, a) {
							return true
						}
					}
					return false
				}
				for _, a := range legal {
					if isSys(a.Kind) {
						t.Fatalf("seed %d: ação sys listada: %+v", seed, a)
					}
					if _, _, err := Apply(g, a); err != nil {
						t.Fatalf("seed %d fase %s: listada mas Apply recusa %+v: %v", seed, g.Phase, a, err)
					}
				}
				for _, a := range exhaustiveActions(g, s) {
					if _, _, err := Apply(g, a); err == nil && !inList(a) {
						t.Fatalf("seed %d fase %s: Apply aceita %+v fora da lista", seed, g.Phase, a)
					}
				}
				if !reflect.DeepEqual(legal, LegalActions(g, s)) {
					t.Fatalf("seed %d: ordem não determinística", seed)
				}
				legalAll = append(legalAll, legal...)
			}
			if g.Phase == PhaseWaiting || g.Phase == PhaseHandEnd {
				g, _, _ = Apply(g, Action{Kind: ActSysStartHand})
				continue
			}
			if len(legalAll) == 0 {
				t.Fatalf("seed %d fase %s: ninguém tem ação legal", seed, g.Phase)
			}
			a := legalAll[rng.IntN(len(legalAll))]
			next, _, err := Apply(g, a)
			if err != nil {
				t.Fatalf("seed %d: %v", seed, err)
			}
			g = next
		}
	}
	for _, k := range []HandKind{HandNormal, HandEleven, HandIron} {
		if !kinds[k] {
			t.Errorf("tipo de mão %v não exercitado", k)
		}
	}
	for _, p := range []Phase{PhaseCutChoice, PhaseDealerChoice, PhaseBurnFours, PhasePlaying, PhaseAwaitingRaiseResponse, PhaseElevenDecision} {
		if !phases[p] {
			t.Errorf("fase %s não exercitada", p)
		}
	}
}

func TestLegalActionsEmptyWithoutDecision(t *testing.T) {
	g := NewGame(testPlayers, 3, 0)
	for s := Seat(0); s < 4; s++ {
		if got := LegalActions(g, s); got != nil {
			t.Fatalf("waiting seat %d: %+v", s, got)
		}
	}
	g, _, _ = Apply(g, Action{Kind: ActSysStartHand})
	for step := 0; g.Phase != PhasePlaying && step < 50; step++ {
		for s := Seat(0); s < 4; s++ {
			if acts := LegalActions(g, s); len(acts) > 0 {
				g, _, _ = Apply(g, acts[0])
				break
			}
		}
	}
	if g.Phase != PhasePlaying {
		t.Skip("não chegou a playing")
	}
	for s := Seat(0); s < 4; s++ {
		got := LegalActions(g, s)
		if s == g.Hand.Turn && len(got) == 0 {
			t.Fatalf("assento da vez sem ações")
		}
		if s != g.Hand.Turn && got != nil {
			t.Fatalf("assento %d sem vez devia ser nil: %+v", s, got)
		}
	}
	if LegalActions(g, 4) != nil {
		t.Fatal("assento inválido devia ser nil")
	}
}
