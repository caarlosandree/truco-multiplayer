package engine

import "testing"

func c(r Rank, s Suit) Card { return Card{Rank: r, Suit: s} }

func pc(seat Seat, r Rank, s Suit, covered bool) PlayedCard {
	return PlayedCard{Seat: seat, Card: c(r, s), Covered: covered}
}

func TestManilhaRankIsNextRankCyclically(t *testing.T) {
	cases := map[Rank]Rank{4: 5, 5: 6, 6: 7, 7: 10, 10: 11, 11: 12, 12: 1, 1: 2, 2: 3, 3: 4}
	for vira, want := range cases {
		if got := ManilhaRank(c(vira, Hearts)); got != want {
			t.Errorf("vira %d: manilha = %d, quer %d", vira, got, want)
		}
	}
}

func TestStrengthOrdering(t *testing.T) {
	const manilha Rank = 5
	ordered := []struct {
		name    string
		card    Card
		covered bool
	}{
		{"coberta", c(3, Clubs), true},
		{"4 comum", c(4, Clubs), false},
		{"7 comum", c(7, Hearts), false},
		{"rei", c(12, Spades), false},
		{"ás", c(1, Diamonds), false},
		{"2", c(2, Clubs), false},
		{"3", c(3, Diamonds), false},
		{"manilha ouros", c(5, Diamonds), false},
		{"manilha espadas", c(5, Spades), false},
		{"manilha copas", c(5, Hearts), false},
		{"manilha paus", c(5, Clubs), false},
	}
	for i := 1; i < len(ordered); i++ {
		weaker, stronger := ordered[i-1], ordered[i]
		ws := Strength(weaker.card, weaker.covered, manilha)
		ss := Strength(stronger.card, stronger.covered, manilha)
		if ss <= ws {
			t.Errorf("%s (%d) deveria vencer %s (%d)", stronger.name, ss, weaker.name, ws)
		}
	}
}

func TestCommonCardsOfSameRankTie(t *testing.T) {
	if Strength(c(3, Clubs), false, 5) != Strength(c(3, Diamonds), false, 5) {
		t.Fatal("comuns de mesmo rank deveriam empatar")
	}
}

func TestRoundWinner(t *testing.T) {
	const manilha Rank = 5 // vira 4
	cases := []struct {
		name     string
		table    []PlayedCard
		wantTeam *Team
		wantSeat *Seat
	}{
		{
			name:  "empate entre reis de duplas diferentes",
			table: []PlayedCard{pc(0, 12, Hearts, false), pc(1, 12, Spades, false), pc(2, 4, Diamonds, false), pc(3, 6, Clubs, false)},
		},
		{
			name:     "manilha de paus vence manilha de ouros",
			table:    []PlayedCard{pc(0, 5, Diamonds, false), pc(1, 5, Clubs, false), pc(2, 3, Hearts, false), pc(3, 4, Spades, false)},
			wantTeam: teamPtr(1), wantSeat: seatPtr(1),
		},
		{
			name:     "coberta perde para qualquer aberta",
			table:    []PlayedCard{pc(0, 3, Clubs, true), pc(1, 4, Diamonds, false), pc(2, 2, Clubs, true), pc(3, 1, Clubs, true)},
			wantTeam: teamPtr(1), wantSeat: seatPtr(1),
		},
		{
			name:  "todas cobertas empatam",
			table: []PlayedCard{pc(0, 3, Clubs, true), pc(1, 4, Diamonds, true), pc(2, 2, Clubs, true), pc(3, 1, Clubs, true)},
		},
		{
			name:     "vence o assento do parceiro com a maior",
			table:    []PlayedCard{pc(0, 7, Diamonds, false), pc(1, 4, Hearts, false), pc(2, 3, Spades, false), pc(3, 6, Hearts, false)},
			wantTeam: teamPtr(0), wantSeat: seatPtr(2),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			team, seat := RoundWinner(tc.table, manilha)
			if !equalPtr(team, tc.wantTeam) || !equalPtr(seat, tc.wantSeat) {
				t.Fatalf("RoundWinner = (%v, %v), quer (%v, %v)", deref(team), deref(seat), deref(tc.wantTeam), deref(tc.wantSeat))
			}
		})
	}
}

func equalPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}
