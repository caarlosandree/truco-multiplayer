package room

import (
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
)

// Msg é uma mensagem da caixa de entrada da sala.
type Msg interface{ roomMsg() }

// Join pede um assento; o lobby já reservou o usuário para esta sala.
type Join struct {
	UserID      string
	DisplayName string
	Seat        *engine.Seat
	Seq         int64
}

// Command é uma mensagem do cliente para a sala (assentos, jogo, W.O., sinais).
type Command struct {
	UserID  string
	Message protocol.ClientMessage
}

type Connected struct{ UserID string }

type Disconnected struct{ UserID string }

// Abort encerra a partida viva por decisão do administrador.
type Abort struct{ Reason string }

// Stop encerra a sala no desligamento do servidor, sem tocar na partida (o Recover a retoma).
type Stop struct{}

// Wake só faz a sala reavaliar prazos e timers (usado ao iniciar o ator).
type Wake struct{}

type tick struct{ gen uint64 }

func (Join) roomMsg()         {}
func (Command) roomMsg()      {}
func (Connected) roomMsg()    {}
func (Disconnected) roomMsg() {}
func (Abort) roomMsg()        {}
func (Stop) roomMsg()         {}
func (Wake) roomMsg()         {}
func (tick) roomMsg()         {}
