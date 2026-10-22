// Package room implementa a sala: assentos, partida, timers e presença. Não conhece rede nem
// banco concreto; o ator (actor.go) garante que Handle seja chamado por uma goroutine de cada vez.
package room

import (
	"context"
	"log/slog"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
)

type seatState struct {
	userID         string
	name           string
	ready          bool
	connected      bool
	disconnectedAt time.Time
	timeouts       int           // timeouts seguidos
	absentSince    time.Time     // zero = presente
	absentTotal    time.Duration // ausência acumulada na partida, sem o trecho corrente
	lastSignal     time.Time
	bot            bool // cadeira de bot: sempre conectado e pronto, nunca host, fora do índice do lobby
}

type Room struct {
	opt    Options
	ctx    context.Context
	status protocol.RoomStatus
	host   engine.Seat
	seats  [4]*seatState
	closed bool

	matchID    string
	game       engine.GameState
	nextPly    int
	window     string                    // identifica a janela de decisão corrente
	deadlines  map[engine.Seat]time.Time // prazo de cada assento pendente
	pauseUntil time.Time                 // fim da pausa entre mãos
	offered    *engine.Team              // dupla contra a qual o W.O. está disponível

	timer clock.Timer
	gen   uint64
}

func New(o Options) *Room {
	if o.Log == nil {
		o.Log = slog.New(slog.DiscardHandler)
	}
	return &Room{opt: o, ctx: context.Background(), status: protocol.RoomWaiting}
}

func (r *Room) Closed() bool { return r.closed }

func (r *Room) Status() protocol.RoomStatus { return r.status }

// Game devolve uma cópia do estado da partida (testes e diagnóstico).
func (r *Room) Game() engine.GameState { return r.game.Clone() }

func (r *Room) now() time.Time { return r.opt.Clock.Now() }

// Handle processa uma mensagem e depois reavalia prazos e timers.
func (r *Room) Handle(ctx context.Context, m Msg) {
	if r.closed {
		r.rejectClosed(m)
		return
	}
	r.ctx = ctx
	switch m := m.(type) {
	case Join:
		r.join(m)
	case Vacate:
		r.vacate(m)
	case Command:
		r.command(m)
	case Connected:
		r.connected(m.UserID)
	case Disconnected:
		r.disconnected(m.UserID)
	case Abort:
		r.abort(m)
	case Stop:
		r.stop()
		return
	case tick:
		if m.gen != r.gen {
			return // disparo de um timer já substituído
		}
	case Wake:
	}
	r.settle()
}

func (r *Room) rejectClosed(m Msg) {
	switch m := m.(type) {
	case Join:
		code := protocol.CodeRoomNotFound
		if m.Invite {
			code = protocol.CodeInviteExpired
		}
		r.opt.Out.Send(m.UserID, protocol.ErrorMsg(code, m.Seq))
		r.opt.Registry.Released(r.opt.ID, m.UserID)
	case Vacate:
		r.opt.Registry.Released(r.opt.ID, m.UserID) // o lobby não pode esperar para sempre
	case Command:
		r.opt.Out.Send(m.UserID, protocol.ErrorMsg(protocol.CodeRoomNotFound, m.Message.Seq))
	}
}

func (r *Room) stop() {
	r.closed = true
	r.stopTimer()
}
