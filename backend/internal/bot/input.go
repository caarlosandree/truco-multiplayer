// Package bot contém as políticas de decisão dos bots. Importa só a stdlib e o
// engine, e nenhuma função exportada recebe o GameState de uma partida real:
// o bot só enxerga o que o assento enxergaria (PlayerView e eventos filtrados).
package bot

import (
	"context"
	"math/rand/v2"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// Signal é um sinal de mesa. Os valores são iguais aos de protocol.SignalID.
type Signal string

const (
	SigTruco    Signal = "truco"
	SigBoa      Signal = "boa"
	SigVamos    Signal = "vamos"
	SigCalma    Signal = "calma"
	SigDesculpa Signal = "desculpa"
	SigValeu    Signal = "valeu"
)

// SignalSeen é um sinal visto na mesa, com o assento de origem e a mão em que saiu.
type SignalSeen struct {
	Seat       engine.Seat
	Signal     Signal
	HandNumber int
}

// Input é tudo o que o bot sabe para decidir. Events são os do assento na
// partida inteira, já filtrados; Legal vem de engine.LegalActions, calculado
// pela sala.
type Input struct {
	View    engine.PlayerView
	Events  []engine.Event
	Signals []SignalSeen
	Legal   []engine.Action
}

// Decision é a ação escolhida e, opcionalmente, um sinal para o parceiro.
type Decision struct {
	Action engine.Action
	Signal *Signal
}

// Policy decide a ação de um assento. A decisão deve estar sempre em in.Legal.
type Policy interface {
	Decide(ctx context.Context, in Input, rng *rand.Rand) Decision
}
