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
	Events  [4][]engine.Event // eventos visíveis a cada assento, reprocessados das ações gravadas
	Bots    [4]bool           // cadeiras de bot
}

// NewRestored recria a sala de uma partida em andamento: em jogo, todos desconectados
// (os prazos de ausência recomeçam agora) e o anfitrião no assento 0.
func NewRestored(o Options, rs Restored) *Room {
	r := New(o)
	now := r.now()
	r.status, r.matchID, r.game, r.nextPly = protocol.RoomPlaying, rs.MatchID, rs.State, rs.NextPly
	for s, p := range rs.Players {
		if isBot := rs.Bots[s] || p.Bot; isBot {
			r.seats[s] = &seatState{userID: p.UserID, name: p.DisplayName, connected: true, ready: true, bot: true}
			r.startedWithBots = true
			continue
		}
		r.seats[s] = &seatState{userID: p.UserID, name: p.DisplayName, disconnectedAt: now}
	}
	r.host = r.firstHuman()
	r.seatEvents = rs.Events
	r.syncWindow(now)
	return r
}

func (r *Room) storeCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.ctx, storeTimeout)
}

func (r *Room) maybeStart() {
	if r.status != protocol.RoomWaiting || r.occupied() < 4 {
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
		players[s] = replay.PlayerSeat{UserID: st.userID, DisplayName: st.name, Bot: st.bot}
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
	r.resetBotState()
	for _, st := range r.seats {
		st.timeouts, st.absentSince, st.absentTotal = 0, time.Time{}, 0
		r.startedWithBots = r.startedWithBots || st.bot
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
		r.endMatch(protocol.EndAborted, nil, false)
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
	r.endMatch(protocol.EndAborted, nil, false)
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
		appendSeatEvents(&r.seatEvents, seat, evs) // memória de cada assento (os bots só sabem isto)
		if st.bot {
			continue
		}
		if msg, ok := protocol.EventsMessage(evs, seat); ok {
			r.opt.Out.Send(st.userID, msg)
		}
		r.opt.Out.Send(st.userID, r.stateFor(seat))
	}
}

// appendSeatEvents acrescenta à memória do assento só o que ele pode ver. É a única forma de acumular eventos
// por assento, ao vivo e no restore (SeatEventsOf).
func appendSeatEvents(dst *[4][]engine.Event, seat engine.Seat, evs []engine.Event) {
	dst[seat] = append(dst[seat], engine.FilterEvents(evs, seat)...)
}

// SeatEventsOf reprocessa o log e reconstrói os eventos visíveis a cada assento, idênticos aos que a sala
// acumula ao vivo; o restore (Restored.Events) usa isto para os bots verem o mesmo que veriam sem restart.
func SeatEventsOf(rec replay.Record) ([4][]engine.Event, error) {
	var events [4][]engine.Event
	_, err := replay.Walk(rec, func(st replay.Step) error {
		for s := range events {
			appendSeatEvents(&events, engine.Seat(s), st.Events)
		}
		return nil
	})
	return events, err
}

func (r *Room) broadcastState() {
	for s, st := range r.seats {
		if st != nil && !st.bot {
			r.opt.Out.Send(st.userID, r.stateFor(engine.Seat(s)))
		}
	}
}

// finish grava o resultado. Falha de Finish não trava a sala: o Recover conclui no próximo boot.
func (r *Room) finish(last engine.Action) {
	// Conta no ranking só se nenhuma cadeira nasceu de bot e, ao fim, ou foi W.O. ou nenhum humano da dupla
	// VENCEDORA estava com um bot no controle; quem abandonou na dupla perdedora leva a derrota (spec §6.2).
	// Decidido aqui, antes de o estado da sala voltar à espera.
	ranked := !r.startedWithBots
	if ranked && last.Kind != engine.ActSysForfeit && r.game.Winner != nil {
		ranked = !r.humanBotControlledIn(*r.game.Winner)
	}
	ctx, cancel := r.storeCtx()
	if err := r.opt.Store.Finish(ctx, r.matchID, ranked); err != nil {
		r.opt.Log.Error("falha ao finalizar a partida; o Recover conclui no próximo boot", "match_id", r.matchID, "err", err)
	}
	cancel()
	reason := protocol.EndGameOver
	if last.Kind == engine.ActSysForfeit {
		reason = protocol.EndForfeit
	}
	r.endMatch(reason, r.game.Winner, ranked)
}

func (r *Room) endMatch(reason protocol.EndReason, winner *engine.Team, ranked bool) {
	r.opt.Log.Info("partida encerrada", "match_id", r.matchID, "reason", reason, "ranked", ranked)
	r.broadcast(protocol.MatchEnded(reason, winner, r.game.Score, ranked))
	r.resetToWaiting()
}

// resetToWaiting volta a sala para a espera com os mesmos ocupantes (revanche).
func (r *Room) resetToWaiting() {
	now := r.now()
	r.status, r.matchID, r.game, r.nextPly = protocol.RoomWaiting, "", engine.GameState{}, 0
	r.window, r.deadlines, r.pauseUntil, r.offered = "", nil, time.Time{}, nil
	r.resetBotState()
	for _, st := range r.seats {
		if st == nil {
			continue
		}
		st.ready, st.timeouts, st.absentSince, st.absentTotal = st.bot, 0, time.Time{}, 0
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
	r.endMatch(protocol.EndAborted, nil, false)
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

// resetBotState esquece a memória dos bots da partida anterior (eventos, sinais, pedidos em voo).
func (r *Room) resetBotState() {
	r.seatEvents, r.signalsSeen, r.botReqs = [4][]engine.Event{}, nil, map[engine.Seat]*botReq{}
	r.everBotControlled, r.startedWithBots = [4]bool{}, false
}
