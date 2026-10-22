package room

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
)

var (
	errPersist   = errors.New("room: falha ao gravar a ação")
	errMatchGone = errors.New("room: partida encerrada fora da sala")
)

const (
	storeTimeout = 5 * time.Second
	retryDelay   = time.Second // nova tentativa de ação automática depois de falha de gravação
)

// Restored é uma partida em andamento reconstruída pelo Recover.
type Restored struct {
	MatchID string
	Players [4]replay.PlayerSeat
	State   engine.GameState
	NextPly int
}

// NewRestored recria a sala de uma partida em andamento: em jogo, todos desconectados
// (os prazos de ausência recomeçam agora) e o anfitrião no assento 0.
func NewRestored(o Options, rs Restored) *Room {
	r := New(o)
	now := r.now()
	r.status, r.matchID, r.game, r.nextPly = protocol.RoomPlaying, rs.MatchID, rs.State, rs.NextPly
	for s, p := range rs.Players {
		r.seats[s] = &seatState{userID: p.UserID, name: p.DisplayName, disconnectedAt: now}
	}
	r.syncWindow(now)
	return r
}

func (r *Room) storeCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.ctx, storeTimeout)
}

func (r *Room) maybeStart() {
	if r.status != protocol.RoomWaiting || r.members() < 4 {
		return
	}
	for _, st := range r.seats {
		if !st.ready {
			return
		}
	}
	r.startMatch()
}

func (r *Room) startMatch() {
	var buf [9]byte
	if _, err := io.ReadFull(r.opt.Rand, buf[:]); err != nil {
		r.opt.Log.Error("falha ao sortear a partida", "err", err)
		r.failStart("")
		return
	}
	seed, dealer := binary.LittleEndian.Uint64(buf[:8]), engine.Seat(buf[8]%4)
	var players [4]replay.PlayerSeat
	var enginePlayers [4]engine.Player
	for s, st := range r.seats {
		players[s] = replay.PlayerSeat{UserID: st.userID, DisplayName: st.name}
		enginePlayers[s] = engine.Player{ID: st.userID, Name: st.name}
	}
	ctx, cancel := r.storeCtx()
	id, err := r.opt.Store.Create(ctx, seed, dealer, players)
	cancel()
	if err != nil {
		r.opt.Log.Error("falha ao criar a partida", "err", err)
		r.failStart("")
		return
	}
	r.status, r.matchID, r.game, r.nextPly = protocol.RoomPlaying, id, engine.NewGame(enginePlayers, seed, dealer), 0
	for _, st := range r.seats {
		st.timeouts, st.absentSince, st.absentTotal = 0, time.Time{}, 0
	}
	r.opt.Log.Info("partida iniciada", "match_id", id)
	r.publish()
	if err := r.apply(engine.Action{Kind: engine.ActSysStartHand}, replay.SourceSystem); err != nil && !errors.Is(err, errMatchGone) {
		r.failStart(id)
	}
}

// failStart desfaz um início malsucedido: abandona a partida já criada e volta a esperar.
func (r *Room) failStart(matchID string) {
	if matchID != "" {
		ctx, cancel := r.storeCtx()
		if err := r.opt.Store.Abandon(ctx, matchID, "start_failed"); err != nil {
			r.opt.Log.Error("falha ao abandonar partida que não começou", "match_id", matchID, "err", err)
		}
		cancel()
	}
	r.resetToWaiting()
	r.broadcast(protocol.ErrorMsg(protocol.CodeInternal, 0))
}

// apply aplica, grava e só então transmite. Em erro, o estado não muda.
func (r *Room) apply(a engine.Action, src replay.Source) error {
	next, evs, err := engine.Apply(r.game, a)
	if err != nil {
		return err
	}
	ctx, cancel := r.storeCtx()
	err = r.opt.Store.AppendAction(ctx, r.matchID, r.nextPly, a, src)
	cancel()
	if errors.Is(err, ErrMatchClosed) {
		r.opt.Log.Warn("partida encerrada fora da sala", "match_id", r.matchID)
		r.endMatch(protocol.EndAborted, nil)
		return errMatchGone
	}
	if errors.Is(err, ErrLogDiverged) {
		r.opt.Log.Error("log da partida divergiu do estado da sala; abandonando", "match_id", r.matchID, "ply", r.nextPly, "err", err)
		r.abandonDiverged()
		return errMatchGone
	}
	if err != nil {
		r.opt.Log.Error("falha ao gravar ação", "match_id", r.matchID, "ply", r.nextPly, "err", err)
		return errPersist
	}
	scoreChanged := next.Score != r.game.Score
	r.game, r.nextPly = next, r.nextPly+1
	r.syncWindow(r.now())
	r.broadcastGame(evs)
	if scoreChanged {
		r.opt.Registry.Update(r.summary())
	}
	if next.Phase == engine.PhaseGameOver {
		r.finish(a)
	}
	return nil
}

// abandonDiverged encerra como abortada uma partida cujo log não bate com a sala: continuar só repetiria o conflito.
func (r *Room) abandonDiverged() {
	ctx, cancel := r.storeCtx()
	err := r.opt.Store.Abandon(ctx, r.matchID, "diverged")
	cancel()
	if err != nil && !errors.Is(err, ErrMatchClosed) {
		r.opt.Log.Error("falha ao abandonar partida divergente", "match_id", r.matchID, "err", err)
	}
	r.endMatch(protocol.EndAborted, nil)
}

func (r *Room) stateFor(seat engine.Seat) protocol.ServerMessage {
	return protocol.StateMessage(r.game, seat, r.deadlines)
}

func (r *Room) broadcastGame(evs []engine.Event) {
	for s, st := range r.seats {
		if st == nil {
			continue
		}
		seat := engine.Seat(s)
		if msg, ok := protocol.EventsMessage(evs, seat); ok {
			r.opt.Out.Send(st.userID, msg)
		}
		r.opt.Out.Send(st.userID, r.stateFor(seat))
	}
}

func (r *Room) broadcastState() {
	for s, st := range r.seats {
		if st != nil {
			r.opt.Out.Send(st.userID, r.stateFor(engine.Seat(s)))
		}
	}
}

// finish grava o resultado. Falha de Finish não trava a sala: o Recover conclui no próximo boot.
func (r *Room) finish(last engine.Action) {
	ctx, cancel := r.storeCtx()
	// ranked fixo em true até a T10 calcular.
	if err := r.opt.Store.Finish(ctx, r.matchID, true); err != nil {
		r.opt.Log.Error("falha ao finalizar a partida; o Recover conclui no próximo boot", "match_id", r.matchID, "err", err)
	}
	cancel()
	reason := protocol.EndGameOver
	if last.Kind == engine.ActSysForfeit {
		reason = protocol.EndForfeit
	}
	r.endMatch(reason, r.game.Winner)
}

func (r *Room) endMatch(reason protocol.EndReason, winner *engine.Team) {
	r.opt.Log.Info("partida encerrada", "match_id", r.matchID, "reason", reason)
	r.broadcast(protocol.MatchEnded(reason, winner, r.game.Score))
	r.resetToWaiting()
}

// resetToWaiting volta a sala para a espera com os mesmos ocupantes (revanche).
func (r *Room) resetToWaiting() {
	now := r.now()
	r.status, r.matchID, r.game, r.nextPly = protocol.RoomWaiting, "", engine.GameState{}, 0
	r.window, r.deadlines, r.pauseUntil, r.offered = "", nil, time.Time{}, nil
	for _, st := range r.seats {
		if st == nil {
			continue
		}
		st.ready, st.timeouts, st.absentSince, st.absentTotal = false, 0, time.Time{}, 0
		if !st.connected {
			st.disconnectedAt = now
		}
	}
	r.publish()
}

func (r *Room) abort(m Abort) {
	reason := m.Reason
	if r.status != protocol.RoomPlaying || (m.MatchID != "" && m.MatchID != r.matchID) {
		return
	}
	r.opt.Log.Info("partida abortada", "match_id", r.matchID, "reason", reason)
	r.endMatch(protocol.EndAborted, nil)
}

func (r *Room) playerAction(seat engine.Seat, msg protocol.ClientMessage) {
	user := r.seats[seat].userID
	if r.status != protocol.RoomPlaying {
		r.reply(user, protocol.CodeBadRequest, msg.Seq)
		return
	}
	a, err := protocol.DecodeAction(msg, seat)
	if err != nil {
		r.reply(user, protocol.CodeBadRequest, msg.Seq)
		return
	}
	err = r.apply(a, replay.SourcePlayer)
	switch {
	case err == nil:
		if st := r.seats[seat]; st != nil {
			st.timeouts = 0
		}
	case errors.Is(err, errPersist):
		r.reply(user, protocol.CodeInternal, msg.Seq)
	case errors.Is(err, errMatchGone):
	default:
		r.opt.Out.Send(user, protocol.ErrorMessage(err, msg.Seq))
	}
}
