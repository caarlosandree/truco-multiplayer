package room

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

// ErrMatchClosed indica que a partida foi encerrada fora da sala (ex.: abort do administrador).
// O adaptador do lobby traduz match.ErrMatchClosed para este erro.
var ErrMatchClosed = errors.New("room: partida encerrada fora da sala")

// ErrLogDiverged indica que o log gravado diverge do que a sala acredita (ply já gravado com outra ação
// ou fora de sequência). É fatal para a partida. O adaptador do lobby traduz match.ErrPlyConflict e match.ErrPlyGap.
var ErrLogDiverged = errors.New("room: log da partida divergiu do estado da sala")

// MatchStore é o subconjunto de match.Store usado pela sala, que é o único escritor das ações da sua partida.
type MatchStore interface {
	Create(ctx context.Context, seed uint64, firstDealer engine.Seat, players [4]replay.PlayerSeat) (string, error)
	AppendAction(ctx context.Context, id string, ply int, a engine.Action, src replay.Source) error
	Finish(ctx context.Context, id string) error
	Abandon(ctx context.Context, id, reason string) error
}

// Outbox entrega mensagens a um usuário. Nunca bloqueia; usuário sem conexão é ignorado.
type Outbox interface {
	Send(userID string, msg protocol.ServerMessage)
}

// Registry é o lobby visto pela sala.
type Registry interface {
	Update(s Summary)               // o estado público da sala mudou
	Released(roomID, userID string) // o usuário deixou de pertencer à sala
	Destroyed(roomID string)        // a sala fechou e não aceita mais membros
}

// Summary é o que o lobby precisa saber da sala.
type Summary struct {
	Room    protocol.RoomSummary
	Public  bool
	Code    string
	MatchID string
}

// Timing tem os mesmos campos, na mesma ordem, de config.GameTiming (conversão direta).
type Timing struct {
	Play, Vote, Choice, Burn, Absent time.Duration
	HandPause                        time.Duration
	AbsenceGrace, AbsenceBudget      time.Duration
	WaitingDisconnect                time.Duration
}

type Options struct {
	ID       string
	Code     string
	Name     string
	Public   bool
	Timing   Timing
	Store    MatchStore
	Out      Outbox
	Registry Registry
	Clock    clock.Clock
	Rand     io.Reader // seed e dealer de cada partida (crypto/rand em produção)
	Log      *slog.Logger
	Post     func(Msg) // entrega mensagens (disparos de timer) na caixa de entrada do ator
}
