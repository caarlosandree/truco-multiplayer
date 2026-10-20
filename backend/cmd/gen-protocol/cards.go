package main

import (
	"encoding/json"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// cardRow é uma linha da tabela id → carta usada pelo teste de paridade do frontend.
type cardRow struct {
	ID   int `json:"id"`
	Rank int `json:"rank"`
	Suit int `json:"suit"`
}

// renderCards serializa engine.CardFromID para os 40 ids, com indentação estável.
func renderCards() []byte {
	rows := make([]cardRow, engine.DeckSize)
	for i := range rows {
		c := engine.CardFromID(engine.CardID(i))
		rows[i] = cardRow{ID: i, Rank: int(c.Rank), Suit: int(c.Suit)}
	}
	out, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		panic(err) // tipos fixos: não falha
	}
	return append(out, '\n')
}
