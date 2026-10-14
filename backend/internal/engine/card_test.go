package engine

import (
	"slices"
	"testing"
)

func TestCardIDRoundTrip(t *testing.T) {
	for id := range DeckSize {
		c := CardFromID(CardID(id))
		if got := c.ID(); got != CardID(id) {
			t.Fatalf("CardFromID(%d).ID() = %d", id, got)
		}
	}
}

func TestNewDeckHas40UniqueCardsWithout8And9(t *testing.T) {
	deck := NewDeck()
	if len(deck) != DeckSize {
		t.Fatalf("len = %d, quer %d", len(deck), DeckSize)
	}
	seen := map[Card]bool{}
	for _, c := range deck {
		if c.Rank == 0 || c.Rank == 8 || c.Rank == 9 || c.Rank > 12 {
			t.Fatalf("rank inválido no baralho: %+v", c)
		}
		if seen[c] {
			t.Fatalf("carta duplicada: %+v", c)
		}
		seen[c] = true
	}
}

func TestShuffleIsDeterministicPerSeedAndHand(t *testing.T) {
	a, b, c := NewDeck(), NewDeck(), NewDeck()
	Shuffle(a, handRNG(42, 1))
	Shuffle(b, handRNG(42, 1))
	Shuffle(c, handRNG(42, 2))
	if !slices.Equal(a, b) {
		t.Fatal("mesma seed e mão deveriam gerar o mesmo embaralhamento")
	}
	if slices.Equal(a, c) {
		t.Fatal("mãos diferentes deveriam gerar embaralhamentos diferentes")
	}
}
