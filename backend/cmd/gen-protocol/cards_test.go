package main

import (
	"encoding/json"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

func TestRenderCardsCoversWholeDeckInIDOrder(t *testing.T) {
	var got []cardRow
	if err := json.Unmarshal(renderCards(), &got); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if len(got) != engine.DeckSize {
		t.Fatalf("esperava %d cartas, veio %d", engine.DeckSize, len(got))
	}
	for i, row := range got {
		c := engine.CardFromID(engine.CardID(i))
		if row.ID != i || row.Rank != int(c.Rank) || row.Suit != int(c.Suit) {
			t.Fatalf("linha %d = %+v, esperava rank %d naipe %d", i, row, c.Rank, c.Suit)
		}
	}
}
