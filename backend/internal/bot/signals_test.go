package bot

import (
	"context"
	"math/rand/v2"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

func isIntent(s Signal) bool { return s == SigTruco || s == SigVamos || s == SigCalma }

// recPolicy grava os sinais que a política interna devolve.
type recPolicy struct {
	inner Policy
	log   *[]SignalSeen
}

func (r recPolicy) Decide(ctx context.Context, in Input, rng *rand.Rand) Decision {
	d := r.inner.Decide(ctx, in, rng)
	if d.Signal != nil {
		*r.log = append(*r.log, SignalSeen{Seat: in.View.Seat, Signal: *d.Signal, HandNumber: in.View.HandNumber})
	}
	return d
}

func TestAtMostOneIntentSignalPerHand(t *testing.T) {
	// Cenários sintéticos: o mesmo Input repetido, realimentando os sinais emitidos.
	rng := rand.New(rand.NewPCG(5, 5))
	strongHandCards := []engine.Card{card(3, engine.Clubs), card(2, engine.Hearts), card(1, engine.Spades)}
	weakHandCards := []engine.Card{card(4, engine.Clubs), card(5, engine.Hearts), card(7, engine.Spades)}
	for name, tc := range map[string]struct {
		hand []engine.Card
		act  engine.Action
	}{
		"truco-forte":  {strongHandCards, engine.Action{Kind: engine.ActRequestRaise}},
		"jogada-forte": {strongHandCards, engine.Action{Kind: engine.ActPlayCard}},
		"jogada-fraca": {weakHandCards, engine.Action{Kind: engine.ActPlayCard}},
	} {
		t.Run(name, func(t *testing.T) {
			in := Input{View: playingView(engine.HandNormal, tc.hand)}
			in.View.HandNumber = 4
			emitted, intents := 0, 0
			for range 300 {
				s := intentSignal(in, tc.act, rng)
				if s == nil {
					continue
				}
				emitted++
				if isIntent(*s) {
					intents++
				}
				in.Signals = append(in.Signals, SignalSeen{Seat: in.View.Seat, Signal: *s, HandNumber: in.View.HandNumber})
			}
			if intents > 1 {
				t.Fatalf("%d sinais de intenção na mesma mão", intents)
			}
			if emitted > maxSignalsInHand {
				t.Fatalf("%d sinais na mão, teto %d", emitted, maxSignalsInHand)
			}
			if intents == 0 {
				t.Fatalf("nenhum sinal de intenção em 300 chamadas: a regra não dispara")
			}
		})
	}

	// Partidas reais: nenhum assento passa de 1 sinal de intenção por mão, nem do teto de sinais.
	total := 0
	for seed := uint64(1); seed <= 6; seed++ {
		var log []SignalSeen
		p := recPolicy{inner: Strong{Samples: 6}, log: &log}
		PlayMatch([4]Policy{p, p, p, p}, seed)
		type key struct {
			seat engine.Seat
			hand int
		}
		intents, all := map[key]int{}, map[key]int{}
		for _, sg := range log {
			k := key{sg.Seat, sg.HandNumber}
			all[k]++
			if isIntent(sg.Signal) {
				intents[k]++
			}
		}
		for k, c := range intents {
			if c > 1 {
				t.Fatalf("seed %d: assento %d mandou %d sinais de intenção na mão %d", seed, k.seat, c, k.hand)
			}
		}
		for k, c := range all {
			if c > maxSignalsInHand {
				t.Fatalf("seed %d: assento %d mandou %d sinais na mão %d", seed, k.seat, c, k.hand)
			}
		}
		total += len(log)
	}
	if total == 0 {
		t.Fatalf("nenhum sinal em 6 partidas de bots fortes")
	}
}

func votingInput(rng *rand.Rand, signals []SignalSeen) Input {
	for {
		vira := engine.Card{Rank: []engine.Rank{10, 11, 12, 1, 2}[rng.IntN(5)], Suit: engine.Suit(rng.IntN(4))}
		manilha := engine.ManilhaRank(vira)
		used := map[engine.CardID]bool{vira.ID(): true}
		var mine []engine.Card
		hi := randomCard(rng, []engine.Rank{1, 2, 3}, used, manilha)
		used[hi.ID()] = true
		mine = append(mine, hi)
		for len(mine) < 3 {
			c := randomCard(rng, []engine.Rank{4, 5, 6, 7, 10, 11, 12}, used, manilha)
			used[c.ID()] = true
			mine = append(mine, c)
		}
		g := craftedHand(rng, mine, vira)
		team0, team1 := engine.Team(0), engine.Team(1)
		g.Phase = engine.PhaseAwaitingRaiseResponse
		g.Hand.Turn = 1
		g.Hand.Pending = &engine.PendingRaise{Requester: 1, FromTeam: team1, PrevStake: 1, ToStake: 3}
		g.Hand.VotingTeam = &team0
		in := inputFromState(g, 0, nil)
		in.Signals = signals
		if len(in.Legal) > 0 {
			return in
		}
	}
}

// acceptRate é a fração de estados de votação em que o bot não corre.
func acceptRate(signals func(in Input) []SignalSeen, n int) float64 {
	st := Strong{Samples: 60}
	rng := rand.New(rand.NewPCG(77, 78))
	accepted := 0
	for i := range n {
		in := votingInput(rng, nil)
		in.View.HandNumber = 1
		in.Signals = signals(in)
		d := st.Decide(context.Background(), in, rand.New(rand.NewPCG(uint64(i), 4)))
		if d.Action.Kind == engine.ActVote && d.Action.Choice != engine.VoteRun {
			accepted++
		}
	}
	return float64(accepted) / float64(n)
}

func TestPartnerTrucoSignalRaisesWillingness(t *testing.T) {
	const n = 300
	base := acceptRate(func(Input) []SignalSeen { return nil }, n)
	withSig := acceptRate(func(in Input) []SignalSeen {
		return []SignalSeen{{Seat: engine.Partner(in.View.Seat), Signal: SigTruco, HandNumber: in.View.HandNumber}}
	}, n)
	t.Logf("aceita: sem sinal %.3f, com truco do parceiro %.3f", base, withSig)
	if withSig <= base {
		t.Fatalf("sinal truco do parceiro não aumentou a disposição: %.3f -> %.3f", base, withSig)
	}
}

func TestOpponentSignalWeighsLess(t *testing.T) {
	const n = 300
	base := acceptRate(func(Input) []SignalSeen { return nil }, n)
	partner := acceptRate(func(in Input) []SignalSeen {
		return []SignalSeen{{Seat: engine.Partner(in.View.Seat), Signal: SigTruco, HandNumber: in.View.HandNumber}}
	}, n)
	opp := acceptRate(func(in Input) []SignalSeen {
		return []SignalSeen{{Seat: engine.Next(in.View.Seat), Signal: SigTruco, HandNumber: in.View.HandNumber}}
	}, n)
	t.Logf("aceita: base %.3f, parceiro %.3f, adversário %.3f", base, partner, opp)
	dPartner, dOpp := partner-base, opp-base
	if dOpp < 0 {
		dOpp = -dOpp
	}
	if dOpp >= dPartner {
		t.Fatalf("sinal de adversário pesou %.3f, o do parceiro %.3f", dOpp, dPartner)
	}
}

func TestCalmaSignalLowersWillingness(t *testing.T) {
	const n = 300
	base := acceptRate(func(Input) []SignalSeen { return nil }, n)
	calma := acceptRate(func(in Input) []SignalSeen {
		return []SignalSeen{{Seat: engine.Partner(in.View.Seat), Signal: SigCalma, HandNumber: in.View.HandNumber}}
	}, n)
	if calma >= base {
		t.Fatalf("sinal calma do parceiro não reduziu a disposição: %.3f -> %.3f", base, calma)
	}
}

func TestSignalsOfOtherHandsAreIgnored(t *testing.T) {
	const n = 200
	base := acceptRate(func(Input) []SignalSeen { return nil }, n)
	old := acceptRate(func(in Input) []SignalSeen {
		return []SignalSeen{{Seat: engine.Partner(in.View.Seat), Signal: SigTruco, HandNumber: in.View.HandNumber - 1}}
	}, n)
	if old != base {
		t.Fatalf("sinal de mão anterior mudou a decisão: %.3f vs %.3f", base, old)
	}
}

// roundEndedInput monta um Input em que a última rodada acabou de terminar.
func roundEndedInput(me engine.Seat, winnerTeam engine.Team, winnerSeat engine.Seat, rounds []engine.RoundView) Input {
	in := Input{View: playingView(engine.HandNormal, []engine.Card{card(4, engine.Clubs), card(5, engine.Hearts)})}
	in.View.Seat = me
	in.View.HandNumber = 2
	in.View.Rounds = rounds
	ws, wt := winnerSeat, winnerTeam
	in.Events = []engine.Event{
		{Kind: engine.EvHandStarted, Visibility: engine.Visibility{Kind: engine.VisAll}},
		{Kind: engine.EvRoundEnded, Visibility: engine.Visibility{Kind: engine.VisAll}, Team: &wt, Seat: &ws},
	}
	return in
}

func drawSignals(in Input, n int) map[Signal]int {
	rng := rand.New(rand.NewPCG(9, 9))
	out := map[Signal]int{}
	for range n {
		if s := intentSignal(in, engine.Action{Kind: engine.ActPlayCard}, rng); s != nil {
			out[*s]++
		}
	}
	return out
}

func TestSocialReactions(t *testing.T) {
	won := engine.Team(0)
	lost := engine.Team(1)
	one := []engine.RoundView{{Winner: &won}}
	const n = 1000

	// Parceiro (assento 2) ganhou a rodada: só valeu.
	got := drawSignals(roundEndedInput(0, 0, 2, one), n)
	if got[SigValeu] == 0 || got[SigBoa] != 0 || got[SigDesculpa] != 0 || got[SigValeu] > n/2 {
		t.Fatalf("parceiro ganhou: %v", got)
	}
	// O próprio assento ganhou: só boa.
	got = drawSignals(roundEndedInput(0, 0, 0, one), n)
	if got[SigBoa] == 0 || got[SigValeu] != 0 || got[SigBoa] > n/2 {
		t.Fatalf("eu ganhei: %v", got)
	}
	// Adversário ganhou a primeira rodada: nenhuma reação.
	oneLost := []engine.RoundView{{Winner: &lost}}
	if got = drawSignals(roundEndedInput(0, 1, 1, oneLost), n); len(got) != 0 {
		t.Fatalf("perdeu a 1ª rodada: %v", got)
	}
	// Perdeu a 2ª depois de ganhar a 1ª, tendo jogado nela: desculpa.
	r2 := engine.RoundView{Winner: &lost, Cards: []engine.TableCardView{{Seat: 0}, {Seat: 1}}}
	two := []engine.RoundView{{Winner: &won}, r2}
	got = drawSignals(roundEndedInput(0, 1, 1, two), n)
	if got[SigDesculpa] == 0 || got[SigDesculpa] > n/2 {
		t.Fatalf("perdeu a decisiva: %v", got)
	}
	// Já mandei uma reação nesta rodada: não repete.
	in := roundEndedInput(0, 0, 2, one)
	in.Signals = []SignalSeen{{Seat: 0, Signal: SigValeu, HandNumber: 2}}
	if got = drawSignals(in, n); got[SigValeu] != 0 {
		t.Fatalf("repetiu a reação à mesma rodada: %v", got)
	}
}
