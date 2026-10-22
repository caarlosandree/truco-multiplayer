package bot

import (
	"context"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// craftedHand monta uma mão normal em jogo, com o assento 0 na vez: a mão dele
// é a dada, o resto do baralho (menos a vira) é distribuído ao acaso.
func craftedHand(rng *rand.Rand, mine []engine.Card, vira engine.Card) engine.GameState {
	used := map[engine.CardID]bool{vira.ID(): true}
	for _, c := range mine {
		used[c.ID()] = true
	}
	var rest []engine.Card
	for id := range engine.DeckSize {
		if !used[engine.CardID(id)] {
			rest = append(rest, engine.CardFromID(engine.CardID(id)))
		}
	}
	rng.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
	h := &engine.HandState{Kind: engine.HandNormal, Stake: 1, Vira: &vira, Manilha: engine.ManilhaRank(vira), Turn: 0, RoundStarter: 0}
	h.Hands[0] = append([]engine.Card(nil), mine...)
	for s := 1; s < 4; s++ {
		h.Hands[s] = append([]engine.Card(nil), rest[:3]...)
		rest = rest[3:]
	}
	h.Deck = rest
	return engine.GameState{Seed: 1, HandNumber: 1, Players: testPlayers, Dealer: 3, Phase: engine.PhasePlaying, Hand: h}
}

func randomCard(rng *rand.Rand, ranks []engine.Rank, avoid map[engine.CardID]bool, manilha engine.Rank) engine.Card {
	for {
		c := engine.Card{Rank: ranks[rng.IntN(len(ranks))], Suit: engine.Suit(rng.IntN(4))}
		if !avoid[c.ID()] && c.Rank != manilha {
			return c
		}
	}
}

// weakInput sorteia uma mão só de cartas fracas (4, 5, 6 ou 7 sem manilha) para o assento 0.
func weakInput(rng *rand.Rand) Input {
	vira := engine.Card{Rank: []engine.Rank{10, 11, 12, 1, 2}[rng.IntN(5)], Suit: engine.Suit(rng.IntN(4))}
	manilha := engine.ManilhaRank(vira)
	used := map[engine.CardID]bool{vira.ID(): true}
	var mine []engine.Card
	for len(mine) < 3 {
		c := randomCard(rng, []engine.Rank{4, 5, 6, 7}, used, manilha)
		used[c.ID()] = true
		mine = append(mine, c)
	}
	return inputFromState(craftedHand(rng, mine, vira), 0, nil)
}

func TestStrongBluffsSometimes(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 12))
	st := Strong{Samples: 40}
	bluffs := 0
	const hands = 300
	for range hands {
		in := weakInput(rng)
		if st.Decide(context.Background(), in, rng).Action.Kind == engine.ActRequestRaise {
			bluffs++
		}
	}
	t.Logf("blefes: %d/%d", bluffs, hands)
	if bluffs == 0 || bluffs >= hands/2 {
		t.Fatalf("blefes = %d em %d mãos fracas; quer entre 1 e %d", bluffs, hands, hands/2-1)
	}
}

func TestStrongNeverSuicideRaise(t *testing.T) {
	st := Strong{Samples: 6}
	checked := 0
	for _, score := range [][2]int{{11, 5}, {3, 11}, {11, 11}} {
		for i, sn := range randomSnapshotsFrom(t, 170, score) {
			rng := rand.New(rand.NewPCG(uint64(i), 5))
			for seat := engine.Seat(0); seat < 4; seat++ {
				in := inputFromState(sn.g, seat, sn.evs)
				if len(in.Legal) == 0 {
					continue
				}
				d := st.Decide(context.Background(), in, rng)
				if !actionIn(in.Legal, d.Action) {
					t.Fatalf("decisão %+v fora de Legal", d.Action)
				}
				checked++
				_, evs, err := engine.Apply(sn.g, d.Action)
				if err != nil {
					t.Fatalf("Apply(%+v): %v", d.Action, err)
				}
				for _, e := range evs {
					if e.Kind == engine.EvRaisePunished {
						t.Fatalf("aumento punido (placar %v, fase %s): %+v", score, sn.g.Phase, d.Action)
					}
				}
			}
		}
	}
	if checked < 500 {
		t.Fatalf("só %d decisões checadas", checked)
	}
}

func TestStrongStopsOnContextDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), -time.Second)
	defer cancel()
	st := DefaultStrong()
	for i, sn := range randomSnapshots(t, 60) {
		for seat := engine.Seat(0); seat < 4; seat++ {
			in := inputFromState(sn.g, seat, sn.evs)
			if len(in.Legal) == 0 {
				continue
			}
			start := time.Now()
			d := st.Decide(ctx, in, rand.New(rand.NewPCG(uint64(i), 1)))
			if el := time.Since(start); el > 5*time.Millisecond {
				t.Fatalf("com ctx vencido levou %v", el)
			}
			if !actionIn(in.Legal, d.Action) {
				t.Fatalf("decisão %+v fora de Legal", d.Action)
			}
		}
	}
}

func TestStrongReturnsBestSoFarWhenDeadlineHitsMidway(t *testing.T) {
	in := weakInput(rand.New(rand.NewPCG(3, 3)))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Millisecond)
	defer cancel()
	start := time.Now()
	d := Strong{Samples: 5_000_000}.Decide(ctx, in, rand.New(rand.NewPCG(1, 1)))
	if el := time.Since(start); el > 200*time.Millisecond {
		t.Fatalf("não respeitou o prazo: %v", el)
	}
	if !actionIn(in.Legal, d.Action) {
		t.Fatalf("decisão %+v fora de Legal", d.Action)
	}
}

func TestStrongDeterministicWithSeed(t *testing.T) {
	st := Strong{Samples: 30}
	for i, sn := range randomSnapshots(t, 80) {
		for seat := engine.Seat(0); seat < 4; seat++ {
			in := inputFromState(sn.g, seat, sn.evs)
			if len(in.Legal) == 0 {
				continue
			}
			d1 := st.Decide(context.Background(), in, rand.New(rand.NewPCG(uint64(i), 9)))
			d2 := st.Decide(context.Background(), in, rand.New(rand.NewPCG(uint64(i), 9)))
			if !sameAction(d1.Action, d2.Action) || (d1.Signal == nil) != (d2.Signal == nil) {
				t.Fatalf("decisões diferentes com a mesma seed: %+v vs %+v", d1, d2)
			}
		}
	}
}

// Trocar as cartas que o assento não vê não muda a decisão (anti-trapaça).
func TestStrongIgnoresHiddenCards(t *testing.T) {
	st := Strong{Samples: 30}
	rng := rand.New(rand.NewPCG(8, 8))
	for i, sn := range randomSnapshots(t, 80) {
		g := sn.g
		if g.Hand == nil || g.Phase != engine.PhasePlaying {
			continue
		}
		me := g.Hand.Turn
		g2 := g.Clone()
		// Troca as cartas ainda não jogadas de dois outros assentos (o adversário
		// seguinte e o parceiro), slot a slot onde ambos têm carta na mão.
		other, third := engine.Next(me), engine.Partner(me)
		swapped := false
		for slot := range 3 {
			if slot < len(g2.Hand.Hands[other]) && slot < len(g2.Hand.Hands[third]) &&
				!g2.Hand.Played[other][slot] && !g2.Hand.Played[third][slot] {
				g2.Hand.Hands[other][slot], g2.Hand.Hands[third][slot] = g2.Hand.Hands[third][slot], g2.Hand.Hands[other][slot]
				swapped = true
			}
		}
		if !swapped {
			continue
		}
		in1, in2 := inputFromState(g, me, sn.evs), inputFromState(g2, me, sn.evs)
		if len(in1.Legal) == 0 {
			continue
		}
		seed := rng.Uint64()
		d1 := st.Decide(context.Background(), in1, rand.New(rand.NewPCG(seed, 1)))
		d2 := st.Decide(context.Background(), in2, rand.New(rand.NewPCG(seed, 1)))
		if !sameAction(d1.Action, d2.Action) {
			t.Fatalf("snapshot %d: a decisão depende de cartas escondidas: %+v vs %+v", i, d1.Action, d2.Action)
		}
	}
}

type wrapSimple struct{ Simple }

// O simulador rápido tem de reproduzir o engine: o valor de cada ação do rollout
// Simple bate bit a bit com o rollout no engine de verdade.
func TestFastRolloutMatchesEngine(t *testing.T) {
	rng := rand.New(rand.NewPCG(21, 22))
	checked := 0
	var snaps []snapshot
	snaps = append(snaps, randomSnapshots(t, 400)...)
	snaps = append(snaps, randomSnapshotsFrom(t, 150, [2]int{11, 4})...)
	snaps = append(snaps, randomSnapshotsFrom(t, 150, [2]int{11, 11})...)
	snaps = append(snaps, randomSnapshotsFrom(t, 100, [2]int{9, 10})...)
	for _, sn := range snaps {
		for seat := engine.Seat(0); seat < 4; seat++ {
			in := inputFromState(sn.g, seat, sn.evs)
			if len(in.Legal) == 0 || (in.View.Phase != engine.PhasePlaying && in.View.Phase != engine.PhaseAwaitingRaiseResponse && in.View.Phase != engine.PhaseElevenDecision) {
				continue
			}
			g := Sample(in, NewBelief(in), rng)
			base, ok := simFrom(&g)
			if !ok {
				t.Fatalf("simFrom falhou na fase %s", g.Phase)
			}
			team := int8(engine.TeamOf(seat))
			for _, a := range in.Legal {
				s := base
				s.apply(a)
				s.rolloutSimple()
				fast := utility(s.score, team)
				slow := slowRolloutValue(context.Background(), wrapSimple{}, g, a, team, rng)
				if fast != slow {
					t.Fatalf("fase %s ação %+v: rápido %.4f engine %.4f (placar %v)", g.Phase, a, fast, slow, g.Score)
				}
				checked++
			}
		}
	}
	if checked < 1500 {
		t.Fatalf("só %d comparações", checked)
	}
	t.Logf("%d comparações", checked)
}

func TestStrongBurnsFoursAndKeepsBestCut(t *testing.T) {
	st := Strong{Samples: 4}
	found := false
	for _, sn := range randomSnapshots(t, 400) {
		for seat := engine.Seat(0); seat < 4; seat++ {
			in := inputFromState(sn.g, seat, sn.evs)
			if in.View.Phase != engine.PhaseBurnFours || len(in.Legal) == 0 {
				continue
			}
			d := st.Decide(context.Background(), in, rand.New(rand.NewPCG(1, 1)))
			fours := 0
			for _, h := range in.View.MyHand {
				if h.Card != nil && engine.CardFromID(*h.Card).Rank == 4 {
					fours++
				}
			}
			if fours > 0 {
				found = true
				if d.Action.Kind != engine.ActBurn || len(d.Action.Slots) != fours {
					t.Fatalf("com %d quatros devia queimar todos, veio %+v", fours, d.Action)
				}
			}
		}
	}
	if !found {
		t.Skip("nenhum estado de queima com quatros na amostra")
	}
}

func TestStrongAverageDecisionTime(t *testing.T) {
	st := DefaultStrong()
	var total time.Duration
	n := 0
	for i, sn := range randomSnapshots(t, 120) {
		for seat := engine.Seat(0); seat < 4; seat++ {
			in := inputFromState(sn.g, seat, sn.evs)
			if len(in.Legal) == 0 {
				continue
			}
			start := time.Now()
			st.Decide(context.Background(), in, rand.New(rand.NewPCG(uint64(i), 3)))
			total += time.Since(start)
			n++
		}
	}
	t.Logf("tempo médio por decisão com DefaultStrong: %v (%d decisões)", total/time.Duration(n), n)
	if avg := total / time.Duration(n); avg > 200*time.Millisecond {
		t.Fatalf("decisão média lenta: %v", avg)
	}
}
