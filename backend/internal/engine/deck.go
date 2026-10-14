package engine

import "math/rand/v2"

const DeckSize = 40

// NewDeck devolve as 40 cartas em ordem de CardID.
func NewDeck() []Card {
	deck := make([]Card, 0, DeckSize)
	for id := range DeckSize {
		deck = append(deck, CardFromID(CardID(id)))
	}
	return deck
}

func Shuffle(deck []Card, rng *rand.Rand) {
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
}

// handRNG deriva um RNG determinístico por mão a partir da seed da partida.
func handRNG(seed uint64, handNumber int) *rand.Rand {
	return rand.New(rand.NewPCG(seed, uint64(handNumber)))
}
