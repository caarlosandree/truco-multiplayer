package room

import (
	"github.com/caarlosandree/truco-multiplayer/backend/internal/bot"
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
	// Invite marca a entrada por aceite de convite: o lugar pedido ocupado cai no primeiro livre, e sala
	// cheia ou em jogo responde invite_expired (o servidor decide a corrida "sala encheu x aceite").
	Invite bool
}

// Vacate pede que o usuário deixe a sala de espera para aceitar um convite para outra sala.
// Em espera, o usuário sai (left_room{left}) e o lobby é avisado por Registry.Released; em jogo nada muda e
// Refused é chamado. Usuário que já não está sentado também só é liberado no lobby.
type Vacate struct {
	UserID string
	// Refused é chamado só pelo ator da sala (nunca dentro de código que segure o mutex do Lobby),
	// quando a recusa é por partida em andamento.
	Refused func()
}

// Command é uma mensagem do cliente para a sala (assentos, jogo, W.O., sinais).
type Command struct {
	UserID  string
	Message protocol.ClientMessage
}

type Connected struct{ UserID string }

type Disconnected struct{ UserID string }

// Abort encerra a partida viva por decisão do administrador.
// MatchID, se informado, restringe o abort àquela partida (a sala pode já ter iniciado outra).
type Abort struct{ Reason, MatchID string }

// Stop encerra a sala no desligamento do servidor, sem tocar na partida (o Recover a retoma).
type Stop struct{}

// Wake só faz a sala reavaliar prazos e timers (usado ao iniciar o ator).
type Wake struct{}

type tick struct{ gen uint64 }

// botDecided é o resultado do cálculo de um bot, carimbado com a janela e a geração do pedido.
type botDecided struct {
	gen    uint64
	window string
	seat   engine.Seat
	d      bot.Decision
}

func (Join) roomMsg()         {}
func (Vacate) roomMsg()       {}
func (Command) roomMsg()      {}
func (Connected) roomMsg()    {}
func (Disconnected) roomMsg() {}
func (Abort) roomMsg()        {}
func (Stop) roomMsg()         {}
func (Wake) roomMsg()         {}
func (tick) roomMsg()         {}
func (botDecided) roomMsg()   {}
