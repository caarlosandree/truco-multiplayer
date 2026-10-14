package engine

import "fmt"

// Suit é o naipe; o valor numérico é a ordem de desempate entre manilhas.
type Suit uint8

const (
	Diamonds Suit = iota // ♦ Ouros (menor)
	Spades               // ♠ Espadas
	Hearts               // ♥ Copas
	Clubs                // ♣ Paus (maior)
)

// Rank usa valores neutros: 1..7, 10, 11, 12 (o frontend mapeia para A/J/Q/K).
type Rank uint8

// rankLadder é a escada base, da carta mais fraca (índice 0) para a mais forte.
var rankLadder = [10]Rank{4, 5, 6, 7, 10, 11, 12, 1, 2, 3}

func rankIndex(r Rank) int {
	for i, candidate := range rankLadder {
		if candidate == r {
			return i
		}
	}
	panic(fmt.Sprintf("engine: rank inválido %d", r))
}

type Card struct {
	Rank Rank `json:"rank"`
	Suit Suit `json:"suit"`
}

// CardID identifica uma carta em 0..39 (rankIndex*4 + suit).
type CardID uint8

func (c Card) ID() CardID { return CardID(rankIndex(c.Rank)*4 + int(c.Suit)) }

func CardFromID(id CardID) Card {
	return Card{Rank: rankLadder[id/4], Suit: Suit(id % 4)}
}
