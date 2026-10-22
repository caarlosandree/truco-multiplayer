package bot

import (
	"context"
	"math/rand/v2"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

func card(rank engine.Rank, suit engine.Suit) engine.Card { return engine.Card{Rank: rank, Suit: suit} }

func idp(c engine.Card) *engine.CardID { id := c.ID(); return &id }

func seatp(s engine.Seat) *engine.Seat { return &s }

// playingView monta uma visão do assento 0 jogando; vira 5 faz a manilha ser o 6.
func playingView(kind engine.HandKind, hand []engine.Card, table ...engine.TableCardView) engine.PlayerView {
	m := engine.Rank(6)
	v := engine.PlayerView{
		Seat: 0, Phase: engine.PhasePlaying, HandKind: kind, Stake: 1,
		Vira: idp(card(5, engine.Clubs)), Manilha: &m, Turn: seatp(0), Table: table,
	}
	for i, c := range hand {
		v.MyHand = append(v.MyHand, engine.HandSlotView{Slot: i, Card: idp(c)})
	}
	return v
}

func legalPlay(n int) []engine.Action {
	var out []engine.Action
	for i := range n {
		out = append(out, engine.Action{Kind: engine.ActPlayCard, Slot: i}, engine.Action{Kind: engine.ActPlayCard, Slot: i, Covered: true})
	}
	return out
}

func decide(v engine.PlayerView, legal []engine.Action) Decision {
	return Simple{}.Decide(context.Background(), Input{View: v, Legal: legal}, rand.New(rand.NewPCG(1, 1)))
}

func tc(seat engine.Seat, c engine.Card) engine.TableCardView {
	return engine.TableCardView{Seat: seat, Card: idp(c)}
}

func TestSimplePlaysWeakestWinningCard(t *testing.T) {
	hand := []engine.Card{card(4, engine.Clubs), card(2, engine.Spades), card(1, engine.Hearts)}
	// Vira Q: a manilha é o A; o 2♠ é a mais fraca que bate o 7♦ (o A♥ é manilha).
	v := playingView(engine.HandNormal, hand, tc(1, card(7, engine.Diamonds)))
	ace := engine.Rank(1)
	v.Manilha = &ace
	d := decide(v, legalPlay(3))
	if d.Action.Kind != engine.ActPlayCard || d.Action.Slot != 1 || d.Action.Covered {
		t.Fatalf("quer jogar o 2♠ (slot 1) descoberto, veio %+v", d.Action)
	}
	// Vira 5 (manilha 6): sem manilha na mão, o A♥ (mais fraco que o 2♠) já bate o 7♦.
	v = playingView(engine.HandNormal, hand, tc(1, card(7, engine.Diamonds)))
	if d = decide(v, legalPlay(3)); d.Action.Slot != 2 {
		t.Fatalf("quer o A♥ (slot 2), veio %+v", d.Action)
	}
	// Sem carta que ganhe: a mais fraca.
	v = playingView(engine.HandNormal, []engine.Card{card(5, engine.Clubs), card(4, engine.Clubs), card(6, engine.Hearts)}[:2], tc(1, card(3, engine.Diamonds)))
	d = decide(v, legalPlay(2))
	if d.Action.Slot != 1 {
		t.Fatalf("quer a mais fraca (4♣, slot 1), veio %+v", d.Action)
	}
}

func TestSimpleNeverCovers(t *testing.T) {
	hand := []engine.Card{card(4, engine.Clubs), card(2, engine.Spades), card(1, engine.Hearts)}
	for _, table := range [][]engine.TableCardView{nil, {tc(1, card(7, engine.Diamonds))}, {tc(1, card(3, engine.Diamonds))}} {
		d := decide(playingView(engine.HandNormal, hand, table...), legalPlay(3))
		if d.Action.Kind != engine.ActPlayCard || d.Action.Covered {
			t.Fatalf("não pode cobrir: %+v", d.Action)
		}
	}
}

func TestSimpleNeverBurns(t *testing.T) {
	v := engine.PlayerView{Seat: 0, Phase: engine.PhaseBurnFours}
	legal := []engine.Action{{Kind: engine.ActBurn, Slots: []int{0}}, {Kind: engine.ActPassBurn}, {Kind: engine.ActBurn, Slots: []int{0, 1}}}
	if d := decide(v, legal); d.Action.Kind != engine.ActPassBurn {
		t.Fatalf("quer pass_burn, veio %+v", d.Action)
	}
}

func TestSimpleRaisesOnlyWithManilha(t *testing.T) {
	legal := append(legalPlay(3), engine.Action{Kind: engine.ActRequestRaise})
	weak := []engine.Card{card(4, engine.Clubs), card(2, engine.Spades), card(1, engine.Hearts)}
	if d := decide(playingView(engine.HandNormal, weak), legal); d.Action.Kind == engine.ActRequestRaise {
		t.Fatal("pediu truco sem manilha")
	}
	strong := []engine.Card{card(4, engine.Clubs), card(6, engine.Spades), card(1, engine.Hearts)}
	if d := decide(playingView(engine.HandNormal, strong), legal); d.Action.Kind != engine.ActRequestRaise {
		t.Fatalf("quer truco com manilha, veio %+v", d.Action)
	}
	// Mão de 11 e ferro: o pedido seria punido; nunca pede, mesmo com manilha.
	for _, kind := range []engine.HandKind{engine.HandEleven, engine.HandIron} {
		if d := decide(playingView(kind, strong), legal); d.Action.Kind == engine.ActRequestRaise {
			t.Fatalf("pediu aumento suicida em mão %s", kind)
		}
	}
}

func voteView(phase engine.Phase, hand ...engine.Card) engine.PlayerView {
	v := playingView(engine.HandNormal, hand)
	v.Phase = phase
	v.Turn = nil
	return v
}

var voteLegal = []engine.Action{
	{Kind: engine.ActVote, Choice: engine.VoteAccept}, {Kind: engine.ActVote, Choice: engine.VoteRun},
	{Kind: engine.ActVote, Choice: engine.VoteRaise},
}

func TestSimpleAcceptsWithManilhaOrTwoHighCards(t *testing.T) {
	cases := []struct {
		name string
		hand []engine.Card
		want engine.VoteChoice
	}{
		{"manilha", []engine.Card{card(4, engine.Clubs), card(6, engine.Diamonds), card(5, engine.Hearts)}, engine.VoteAccept},
		{"duas altas", []engine.Card{card(4, engine.Clubs), card(2, engine.Spades), card(1, engine.Hearts)}, engine.VoteAccept},
		{"três 3,2,A", []engine.Card{card(3, engine.Clubs), card(2, engine.Spades), card(1, engine.Hearts)}, engine.VoteAccept},
		{"uma alta", []engine.Card{card(4, engine.Clubs), card(2, engine.Spades), card(7, engine.Hearts)}, engine.VoteRun},
		{"fraca", []engine.Card{card(4, engine.Clubs), card(5, engine.Spades), card(7, engine.Hearts)}, engine.VoteRun},
	}
	for _, c := range cases {
		d := decide(voteView(engine.PhaseAwaitingRaiseResponse, c.hand...), voteLegal)
		if d.Action.Kind != engine.ActVote || d.Action.Choice != c.want {
			t.Errorf("%s: quer %s, veio %+v", c.name, c.want, d.Action)
		}
	}
	// Cartas já jogadas (ausentes da MyHand) não contam.
	v := voteView(engine.PhaseAwaitingRaiseResponse, card(2, engine.Spades), card(4, engine.Clubs))
	if d := decide(v, voteLegal); d.Action.Choice != engine.VoteRun {
		t.Errorf("uma alta restante: quer run, veio %+v", d.Action)
	}
}

func TestSimpleElevenPlaysOnlyWithManilha(t *testing.T) {
	legal := []engine.Action{{Kind: engine.ActVote, Choice: engine.VotePlay}, {Kind: engine.ActVote, Choice: engine.VoteRun}}
	with := voteView(engine.PhaseElevenDecision, card(4, engine.Clubs), card(6, engine.Diamonds), card(5, engine.Hearts))
	with.HandKind = engine.HandEleven
	if d := decide(with, legal); d.Action.Choice != engine.VotePlay {
		t.Fatalf("com manilha quer play, veio %+v", d.Action)
	}
	without := voteView(engine.PhaseElevenDecision, card(3, engine.Clubs), card(2, engine.Spades), card(1, engine.Hearts))
	without.HandKind = engine.HandEleven
	if d := decide(without, legal); d.Action.Choice != engine.VoteRun {
		t.Fatalf("sem manilha quer run, veio %+v", d.Action)
	}
}

func TestSimpleNeverSignals(t *testing.T) {
	hand := []engine.Card{card(4, engine.Clubs), card(6, engine.Spades), card(1, engine.Hearts)}
	views := []engine.PlayerView{playingView(engine.HandNormal, hand), voteView(engine.PhaseAwaitingRaiseResponse, hand...)}
	for _, v := range views {
		legal := append(legalPlay(3), engine.Action{Kind: engine.ActRequestRaise})
		if v.Phase != engine.PhasePlaying {
			legal = voteLegal
		}
		if d := decide(v, legal); d.Signal != nil {
			t.Fatalf("Signal deveria ser nil: %v", *d.Signal)
		}
	}
}

func TestSimpleFallsBackToFirstLegalAndHandlesEmpty(t *testing.T) {
	cut := []engine.Action{{Kind: engine.ActChooseCard, Keep: 0}, {Kind: engine.ActChooseCard, Keep: 1}}
	if d := decide(engine.PlayerView{Phase: engine.PhaseCutChoice}, cut); d.Action.Keep != 0 {
		t.Fatalf("corte: primeira ação legal, veio %+v", d.Action)
	}
	if d := decide(engine.PlayerView{}, nil); d.Action.Kind != "" {
		t.Fatalf("sem legal: decisão vazia, veio %+v", d.Action)
	}
}
