package room

import (
	"github.com/caarlosandree/truco-multiplayer/backend/internal/bot"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// InputFor expõe o construtor de bot.Input aos testes externos.
func (r *Room) InputFor(s engine.Seat) bot.Input { return r.inputFor(s) }
