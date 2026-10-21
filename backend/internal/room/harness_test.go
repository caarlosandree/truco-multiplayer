package room_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

var (
	t0         = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	users      = [4]string{"u0", "u1", "u2", "u3"}
	names      = [4]string{"Ana", "Bia", "Caio", "Duda"}
	testTiming = room.Timing{Play: 30 * time.Second, Vote: 20 * time.Second, Choice: 15 * time.Second,
		Burn: 15 * time.Second, Absent: 5 * time.Second, HandPause: 4 * time.Second,
		AbsenceGrace: 2 * time.Minute, AbsenceBudget: 5 * time.Minute, WaitingDisconnect: time.Minute}
)

// fakeStore guarda os logs em memória e valida o contrato de escritor único (ply contíguo).
type fakeStore struct {
	records    map[string]*replay.Record
	order      []string
	abandoned  map[string]string
	failCreate error
	failAppend error
	failFinish error
	onAppend   func()
}

func newFakeStore() *fakeStore {
	return &fakeStore{records: map[string]*replay.Record{}, abandoned: map[string]string{}}
}

func (f *fakeStore) Create(_ context.Context, seed uint64, dealer engine.Seat, players [4]replay.PlayerSeat) (string, error) {
	if f.failCreate != nil {
		return "", f.failCreate
	}
	id := fmt.Sprintf("m%d", len(f.order)+1)
	f.records[id] = &replay.Record{ID: id, Seed: seed, FirstDealer: dealer, EngineVersion: engine.RulesVersion,
		Status: replay.StatusInProgress, Players: players}
	f.order = append(f.order, id)
	return id, nil
}

func (f *fakeStore) AppendAction(_ context.Context, id string, ply int, a engine.Action, src replay.Source) error {
	if f.onAppend != nil {
		f.onAppend()
	}
	if f.failAppend != nil {
		return f.failAppend
	}
	rec := f.records[id]
	if rec.Status != replay.StatusInProgress {
		return room.ErrMatchClosed
	}
	if ply != len(rec.Actions) {
		return fmt.Errorf("ply %d fora de sequência (próximo %d)", ply, len(rec.Actions))
	}
	rec.Actions = append(rec.Actions, replay.LoggedAction{Ply: ply, Action: a, Source: src})
	return nil
}

func (f *fakeStore) Finish(_ context.Context, id string) error {
	if f.failFinish != nil {
		return f.failFinish
	}
	rec := f.records[id]
	final, err := replay.Rebuild(*rec)
	if err != nil {
		return err
	}
	if final.Phase != engine.PhaseGameOver {
		return errors.New("partida não terminou")
	}
	rec.Status = replay.StatusFinished
	return nil
}

func (f *fakeStore) Abandon(_ context.Context, id, reason string) error {
	rec := f.records[id]
	if rec.Status != replay.StatusInProgress {
		return room.ErrMatchClosed
	}
	rec.Status = replay.StatusAbandoned
	f.abandoned[id] = reason
	return nil
}

func (f *fakeStore) last() *replay.Record { return f.records[f.order[len(f.order)-1]] }

func (f *fakeStore) lastAction() replay.LoggedAction {
	acts := f.last().Actions
	return acts[len(acts)-1]
}

// outbox grava as mensagens por usuário; é seguro para uso concorrente (testes do ator).
type outbox struct {
	mu     sync.Mutex
	byUser map[string][]protocol.ServerMessage
}

func newOutbox() *outbox { return &outbox{byUser: map[string][]protocol.ServerMessage{}} }

func (o *outbox) Send(user string, m protocol.ServerMessage) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.byUser[user] = append(o.byUser[user], m)
}

func (o *outbox) all(user string) []protocol.ServerMessage {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]protocol.ServerMessage(nil), o.byUser[user]...)
}

// take devolve e esquece as mensagens do usuário.
func (o *outbox) take(user string) []protocol.ServerMessage {
	o.mu.Lock()
	defer o.mu.Unlock()
	msgs := o.byUser[user]
	delete(o.byUser, user)
	return msgs
}

func (o *outbox) reset() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.byUser = map[string][]protocol.ServerMessage{}
}

func (o *outbox) count(user string, typ protocol.ServerMessageType) int {
	n := 0
	for _, m := range o.all(user) {
		if m.Type == typ {
			n++
		}
	}
	return n
}

func (o *outbox) errorCodes(user string) []string {
	var codes []string
	for _, m := range o.all(user) {
		if m.Type == protocol.MsgError {
			codes = append(codes, m.Payload.(protocol.ErrorPayload).Code)
		}
	}
	return codes
}

// last devolve o payload da última mensagem do tipo recebida pelo usuário.
func last[T any](t *testing.T, o *outbox, user string, typ protocol.ServerMessageType) T {
	t.Helper()
	msgs := o.all(user)
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Type == typ {
			return msgs[i].Payload.(T)
		}
	}
	t.Fatalf("%s não recebeu %s", user, typ)
	var zero T
	return zero
}

func lastCode(t *testing.T, o *outbox, user string) string {
	t.Helper()
	return last[protocol.ErrorPayload](t, o, user, protocol.MsgError).Code
}

type registry struct {
	mu        sync.Mutex
	updates   []room.Summary
	released  []string
	destroyed int
	seated    []string
}

func (r *registry) Seated(_, user string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seated = append(r.seated, user)
}

func (r *registry) wasSeated(user string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.seated {
		if u == user {
			return true
		}
	}
	return false
}

func (r *registry) Update(s room.Summary) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates, s)
}

func (r *registry) Released(_, user string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.released = append(r.released, user)
}

func (r *registry) Destroyed(string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.destroyed++
}

func (r *registry) wasReleased(user string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.released {
		if u == user {
			return true
		}
	}
	return false
}

func (r *registry) lastSummary() room.Summary {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.updates[len(r.updates)-1]
}

// harness dirige uma Room sem goroutines: disparos de timer vão para a fila e são processados em drain.
type harness struct {
	t     *testing.T
	clock *clock.Manual
	store *fakeStore
	out   *outbox
	reg   *registry
	opts  room.Options
	room  *room.Room
	queue []room.Msg
}

func newHarness(t *testing.T, mutate ...func(*room.Options)) *harness {
	h := &harness{t: t, clock: clock.NewManual(t0), store: newFakeStore(), out: newOutbox(), reg: &registry{}}
	o := room.Options{ID: "r1", Code: "K7QM2P", Name: "Sala de Ana", Public: true, Timing: testTiming,
		Store: h.store, Out: h.out, Registry: h.reg, Clock: h.clock, Rand: rand.NewChaCha8([32]byte{1}),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Post: func(m room.Msg) { h.queue = append(h.queue, m) }}
	for _, m := range mutate {
		m(&o)
	}
	h.opts = o
	h.room = room.New(o)
	return h
}

func (h *harness) send(m room.Msg) {
	h.room.Handle(context.Background(), m)
	h.drain()
}

func (h *harness) drain() {
	for len(h.queue) > 0 {
		m := h.queue[0]
		h.queue = h.queue[1:]
		h.room.Handle(context.Background(), m)
	}
}

// advance avança o relógio em passos de no máximo 1s, processando os disparos a cada passo,
// para que prazos armados durante o avanço contem a partir do instante certo.
func (h *harness) advance(d time.Duration) {
	for d > 0 {
		step := min(d, time.Second)
		h.clock.Advance(step)
		h.drain()
		d -= step
	}
}

func (h *harness) cmd(user, typ string, payload any) {
	msg := protocol.ClientMessage{Type: typ, Seq: 7}
	if payload != nil {
		msg.Payload, _ = json.Marshal(payload)
	}
	h.send(room.Command{UserID: user, Message: msg})
}

func (h *harness) join(i int) { h.send(room.Join{UserID: users[i], DisplayName: names[i], Seq: 1}) }

func (h *harness) seatAll() {
	for i := range users {
		h.join(i)
	}
}

func (h *harness) readyAll() {
	for _, u := range users {
		h.cmd(u, protocol.TypeSetReady, protocol.SetReadyPayload{Ready: true})
	}
}

func (h *harness) startMatch() {
	h.t.Helper()
	h.seatAll()
	h.readyAll()
	if h.room.Status() != protocol.RoomPlaying {
		h.t.Fatal("a partida deveria ter começado")
	}
}

func clientMsg(a engine.Action) protocol.ClientMessage {
	msg := protocol.ClientMessage{Type: string(a.Kind), Seq: 3}
	var payload any
	switch a.Kind {
	case engine.ActChooseCard:
		payload = protocol.ChooseCardPayload{Keep: a.Keep}
	case engine.ActBurn:
		payload = protocol.BurnPayload{Slots: a.Slots}
	case engine.ActPlayCard:
		payload = protocol.PlayCardPayload{Slot: a.Slot, Covered: a.Covered}
	case engine.ActVote:
		payload = protocol.VotePayload{Choice: a.Choice}
	}
	if payload != nil {
		msg.Payload, _ = json.Marshal(payload)
	}
	return msg
}

func (h *harness) act(a engine.Action) {
	h.send(room.Command{UserID: users[a.Seat], Message: clientMsg(a)})
}

// nextMove é a jogada "humana" padrão: DefaultAction do menor assento pendente, com carta aberta
// (só cartas cobertas empatariam todas as rodadas e a partida não andaria).
func nextMove(g engine.GameState) (engine.Action, bool) {
	for s := engine.Seat(0); s < 4; s++ {
		if a, ok := engine.DefaultAction(g, s); ok {
			if a.Kind == engine.ActPlayCard {
				a.Covered = false
			}
			return a, true
		}
	}
	return engine.Action{}, false
}

// playUntil joga por todos até cond valer ou a partida acabar; a pausa entre mãos avança o relógio.
func (h *harness) playUntil(cond func(engine.GameState) bool) {
	h.t.Helper()
	for range 5000 {
		if h.room.Status() != protocol.RoomPlaying || h.room.Closed() {
			return
		}
		g := h.room.Game()
		if cond(g) {
			return
		}
		if g.Phase == engine.PhaseHandEnd {
			h.advance(testTiming.HandPause)
			continue
		}
		a, ok := nextMove(g)
		if !ok {
			h.t.Fatalf("nenhum assento pendente na fase %s", g.Phase)
		}
		h.act(a)
	}
	h.t.Fatal("a partida não chegou à condição")
}
