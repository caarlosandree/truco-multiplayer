package bot

import (
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

var testPlayers = [4]engine.Player{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}, {ID: "c", Name: "C"}, {ID: "d", Name: "D"}}

// snapshot é um estado real no meio de uma partida aleatória, com os eventos até ali.
type snapshot struct {
	g   engine.GameState
	evs []engine.Event
}

// randomSnapshots joga partidas aleatórias (ações legais) e guarda n estados.
func randomSnapshots(t *testing.T, n int) []snapshot {
	t.Helper()
	return randomSnapshotsFrom(t, n, [2]int{})
}

// randomSnapshotsFrom começa as partidas no placar dado (11x11 = mão de ferro,
// 11x5 = mão de 11), para cobrir os tipos de mão raros.
func randomSnapshotsFrom(t *testing.T, n int, score [2]int) []snapshot {
	t.Helper()
	var out []snapshot
	for seed := uint64(1); len(out) < n; seed++ {
		rng := rand.New(rand.NewPCG(seed, 99))
		g := engine.NewGame(testPlayers, seed, engine.Seat(seed%4))
		g.Score = score
		var evs []engine.Event
		for step := 0; g.Phase != engine.PhaseGameOver && len(out) < n; step++ {
			if step > 5000 {
				t.Fatalf("seed %d: partida não terminou", seed)
			}
			if step%7 == 3 {
				out = append(out, snapshot{g: g, evs: append([]engine.Event(nil), evs...)})
			}
			a := engine.Action{Kind: engine.ActSysStartHand}
			var opts []engine.Action
			for s := engine.Seat(0); s < 4; s++ {
				opts = append(opts, engine.LegalActions(g, s)...)
			}
			if len(opts) > 0 {
				a = opts[rng.IntN(len(opts))]
			}
			next, newEvs, err := engine.Apply(g, a)
			if err != nil {
				t.Fatalf("seed %d: Apply(%+v): %v", seed, a, err)
			}
			g, evs = next, append(evs, newEvs...)
		}
	}
	return out
}

func checkConservation(t *testing.T, real, s engine.GameState) {
	t.Helper()
	if (real.Hand == nil) != (s.Hand == nil) {
		t.Fatalf("Hand nil diverge")
	}
	if real.Hand == nil {
		return
	}
	rh, sh := real.Hand, s.Hand
	seen := map[engine.CardID]bool{}
	add := func(where string, cards []engine.Card) {
		for _, c := range cards {
			if seen[c.ID()] {
				t.Fatalf("carta duplicada %+v (%s)", c, where)
			}
			seen[c.ID()] = true
		}
	}
	for seat := range 4 {
		if len(sh.Hands[seat]) != len(rh.Hands[seat]) {
			t.Fatalf("mão %d com %d cartas, real %d", seat, len(sh.Hands[seat]), len(rh.Hands[seat]))
		}
		add("hand", sh.Hands[seat])
	}
	add("deck", sh.Deck)
	add("discarded", sh.Discarded)
	add("cutoffer", sh.CutOffer)
	if sh.Vira != nil {
		add("vira", []engine.Card{*sh.Vira})
	}
	if len(seen) != engine.DeckSize {
		t.Fatalf("%d cartas distintas, quer %d (fase %s)", len(seen), engine.DeckSize, real.Phase)
	}
	if len(sh.Deck) != len(rh.Deck) || len(sh.Discarded) != len(rh.Discarded) || len(sh.CutOffer) != len(rh.CutOffer) {
		t.Fatalf("tamanhos: deck %d/%d descarte %d/%d oferta %d/%d", len(sh.Deck), len(rh.Deck),
			len(sh.Discarded), len(rh.Discarded), len(sh.CutOffer), len(rh.CutOffer))
	}
	for _, d := range sh.Discarded {
		if d.Rank != 4 {
			t.Fatalf("queimada não é quatro: %+v", d)
		}
	}
	// Toda carta de mesa/rodada está na mão do assento, no slot marcado como jogado.
	inHand := func(p engine.PlayedCard) {
		for slot, c := range sh.Hands[p.Seat] {
			if c == p.Card && sh.Played[p.Seat][slot] {
				return
			}
		}
		t.Fatalf("carta jogada %+v sem slot jogado na mão", p)
	}
	for _, p := range sh.Table {
		inHand(p)
	}
	for _, r := range sh.Rounds {
		for _, p := range r.Cards {
			inHand(p)
		}
	}
	for seat := range 4 {
		if sh.Played[seat] != rh.Played[seat] && real.Hand != nil && seat >= 0 {
			// os slots jogados podem diferir em posição, mas não em quantidade
			n1, n2 := 0, 0
			for i := range 3 {
				if sh.Played[seat][i] {
					n1++
				}
				if rh.Played[seat][i] {
					n2++
				}
			}
			if n1 != n2 {
				t.Fatalf("assento %d: %d jogadas, real %d", seat, n1, n2)
			}
		}
	}
}

func TestSampleConservesCards(t *testing.T) {
	snaps := randomSnapshots(t, 500)
	phases := map[engine.Phase]int{}
	kinds := map[engine.HandKind]int{}
	rng := rand.New(rand.NewPCG(5, 6))
	for i, sn := range snaps {
		seat := engine.Seat(i % 4)
		in := inputFor(sn.g, seat, sn.evs)
		b := NewBelief(in)
		phases[sn.g.Phase]++
		if sn.g.Hand != nil {
			kinds[sn.g.Hand.Kind]++
		}
		for range 20 {
			checkConservation(t, sn.g, Sample(in, b, rng))
		}
	}
	t.Logf("fases: %v tipos: %v", phases, kinds)
}

func TestSampleMatchesView(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 12))
	for i, sn := range randomSnapshots(t, 500) {
		seat := engine.Seat(i % 4)
		in := inputFor(sn.g, seat, sn.evs)
		b := NewBelief(in)
		for range 5 {
			got := engine.ViewFor(Sample(in, b, rng), seat)
			if !reflect.DeepEqual(got, in.View) {
				t.Fatalf("visão diverge (fase %s, assento %d)\n got: %+v\nwant: %+v", sn.g.Phase, seat, got, in.View)
			}
		}
	}
}

func TestSampleKeepsHiddenHidden(t *testing.T) {
	checked := 0
	snaps := append(randomSnapshots(t, 500), randomSnapshotsFrom(t, 200, [2]int{11, 11})...)
	for i, sn := range snaps {
		h := sn.g.Hand
		if h == nil || len(h.Deck) == 0 {
			continue
		}
		me := engine.Seat(i % 4)
		// Troca cartas não jogadas dos outros assentos (e, na mão de ferro, as
		// próprias, que também são ocultas) com cartas do baralho.
		g2 := sn.g.Clone()
		swapped := false
		for s := engine.Seat(0); s < 4; s++ {
			if s == me && h.Kind != engine.HandIron {
				continue
			}
			if s == engine.Partner(me) && ViewShowsPartner(sn.g, me) {
				continue
			}
			for slot := range g2.Hand.Hands[s] {
				if !g2.Hand.Played[s][slot] && len(g2.Hand.Deck) > 0 {
					d := len(g2.Hand.Deck) - 1 - int(s)%len(g2.Hand.Deck)
					g2.Hand.Hands[s][slot], g2.Hand.Deck[d] = g2.Hand.Deck[d], g2.Hand.Hands[s][slot]
					swapped = true
					break
				}
			}
		}
		if !swapped {
			continue
		}
		in1, in2 := inputFor(sn.g, me, sn.evs), inputFor(g2, me, sn.evs)
		if !reflect.DeepEqual(in1, in2) {
			t.Fatalf("Input depende de carta escondida (fase %s)", sn.g.Phase)
		}
		s1 := Sample(in1, NewBelief(in1), rand.New(rand.NewPCG(3, 4)))
		s2 := Sample(in2, NewBelief(in2), rand.New(rand.NewPCG(3, 4)))
		if !reflect.DeepEqual(s1, s2) {
			t.Fatal("mesma Input e mesma seed deveriam dar a mesma amostra")
		}
		checked++
	}
	if checked < 100 {
		t.Fatalf("poucos casos verificados: %d", checked)
	}
}

// ViewShowsPartner diz se a visão de me inclui a mão do parceiro.
func ViewShowsPartner(g engine.GameState, me engine.Seat) bool {
	return engine.ViewFor(g, me).PartnerHand != nil
}

func TestSampleIronAndElevenHands(t *testing.T) {
	rng := rand.New(rand.NewPCG(31, 32))
	for name, score := range map[string][2]int{"ferro": {11, 11}, "onze": {11, 5}, "onze-b": {3, 11}} {
		kinds := map[engine.HandKind]int{}
		for i, sn := range randomSnapshotsFrom(t, 300, score) {
			seat := engine.Seat(i % 4)
			in := inputFor(sn.g, seat, sn.evs)
			b := NewBelief(in)
			if sn.g.Hand != nil {
				kinds[sn.g.Hand.Kind]++
			}
			for range 5 {
				s := Sample(in, b, rng)
				checkConservation(t, sn.g, s)
				if got := engine.ViewFor(s, seat); !reflect.DeepEqual(got, in.View) {
					t.Fatalf("%s: visão diverge (fase %s)\n got: %+v\nwant: %+v", name, sn.g.Phase, got, in.View)
				}
			}
		}
		if name == "ferro" && kinds[engine.HandIron] == 0 || name != "ferro" && kinds[engine.HandEleven] == 0 {
			t.Fatalf("%s: sem cobertura do tipo de mão: %v", name, kinds)
		}
	}
}

func TestSampleDeterministic(t *testing.T) {
	for _, sn := range randomSnapshots(t, 50) {
		in := inputFor(sn.g, 2, sn.evs)
		b := NewBelief(in)
		a := Sample(in, b, rand.New(rand.NewPCG(8, 9)))
		c := Sample(in, b, rand.New(rand.NewPCG(8, 9)))
		if !reflect.DeepEqual(a, c) {
			t.Fatal("amostra não é determinística")
		}
	}
}

func TestSampleZeroBeliefFallsBackToUniform(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for _, sn := range randomSnapshots(t, 100) {
		in := inputFor(sn.g, 1, sn.evs)
		checkConservation(t, sn.g, Sample(in, Belief{}, rng))
	}
}

// raiseSnapshot acha um estado logo após o assento 1 pedir aumento, visto do assento viewer.
func raiseSnapshot(t *testing.T, viewer engine.Seat) Input {
	t.Helper()
	for seed := uint64(1); seed < 500; seed++ {
		rng := rand.New(rand.NewPCG(seed, 5))
		g := engine.NewGame(testPlayers, seed, 0)
		var evs []engine.Event
		for step := 0; g.Phase != engine.PhaseGameOver && step < 3000; step++ {
			a := engine.Action{Kind: engine.ActSysStartHand}
			var opts []engine.Action
			for s := engine.Seat(0); s < 4; s++ {
				opts = append(opts, engine.LegalActions(g, s)...)
			}
			if len(opts) > 0 {
				a = opts[rng.IntN(len(opts))]
			}
			next, newEvs, err := engine.Apply(g, a)
			if err != nil {
				t.Fatal(err)
			}
			g, evs = next, append(evs, newEvs...)
			if a.Kind == engine.ActRequestRaise && a.Seat == 1 && g.Phase == engine.PhaseAwaitingRaiseResponse {
				return inputFor(g, viewer, evs)
			}
		}
	}
	t.Fatal("nenhum pedido de aumento do assento 1 encontrado")
	return Input{}
}

func TestBeliefRaiserHoldsStrongerCards(t *testing.T) {
	for _, viewer := range []engine.Seat{0, 2, 3} {
		in := raiseSnapshot(t, viewer)
		plain := in
		plain.Events = nil
		raised, uniform := NewBelief(in), NewBelief(plain)
		if got, base := raised.expectedStrength(1), uniform.expectedStrength(1); got <= base {
			t.Fatalf("viewer %d: força esperada %.3f deveria superar a uniforme %.3f", viewer, got, base)
		}
	}
}

func TestSampleFollowsBelief(t *testing.T) {
	in := raiseSnapshot(t, 0)
	plain := in
	plain.Events = nil
	mean := func(b Belief) float64 {
		rng := rand.New(rand.NewPCG(21, 22))
		var sum float64
		const n = 400
		for range n {
			s := Sample(in, b, rng)
			h := s.Hand
			for slot, c := range h.Hands[1] {
				if !h.Played[1][slot] {
					sum += float64(engine.Strength(c, false, h.Manilha)) / 3
				}
			}
		}
		return sum / n
	}
	if raised, base := mean(NewBelief(in)), mean(NewBelief(plain)); raised <= base {
		t.Fatalf("amostras do pedinte (%.3f) deveriam ser mais fortes que as uniformes (%.3f)", raised, base)
	}
}

func TestBeliefPartnerSignalWeighsMoreThanOpponent(t *testing.T) {
	in := raiseSnapshot(t, 0)
	in.Events = nil
	partnerSig, oppSig := in, in
	partnerSig.Signals = []SignalSeen{{Seat: 2, Signal: SigTruco, HandNumber: in.View.HandNumber}}
	oppSig.Signals = []SignalSeen{{Seat: 1, Signal: SigTruco, HandNumber: in.View.HandNumber}}
	base := NewBelief(in)
	p, o := NewBelief(partnerSig), NewBelief(oppSig)
	dp, do := p.expectedStrength(2)-base.expectedStrength(2), o.expectedStrength(1)-base.expectedStrength(1)
	if dp <= do || do <= 0 {
		t.Fatalf("sinal do parceiro (%.3f) deve pesar mais que o do adversário (%.3f), ambos positivos", dp, do)
	}
	stale := in
	stale.Signals = []SignalSeen{{Seat: 2, Signal: SigTruco, HandNumber: in.View.HandNumber - 1}}
	if NewBelief(stale).expectedStrength(2) != base.expectedStrength(2) {
		t.Fatal("sinal de mão anterior não conta")
	}
}

func BenchmarkSample(b *testing.B) {
	t := &testing.T{}
	snaps := randomSnapshots(t, 200)
	var ins []Input
	for i, sn := range snaps {
		if sn.g.Phase == engine.PhasePlaying {
			ins = append(ins, inputFor(sn.g, engine.Seat(i%4), sn.evs))
		}
	}
	bels := make([]Belief, len(ins))
	for i := range ins {
		bels[i] = NewBelief(ins[i])
	}
	rng := rand.New(rand.NewPCG(1, 1))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Sample(ins[i%len(ins)], bels[i%len(ins)], rng)
	}
}
