package room_test

import (
	"context"
	"io"
	"log/slog"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
)

func actorOptions(out room.Outbox, reg *registry) room.Options {
	return room.Options{ID: "r1", Code: "K7QM2P", Name: "Sala de Ana", Public: true, Timing: testTiming,
		Store: newFakeStore(), Out: out, Registry: reg, Clock: clock.NewManual(t0), Rand: rand.NewChaCha8([32]byte{1}),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func waitDone(t *testing.T, a *room.Actor) {
	t.Helper()
	select {
	case <-a.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("o ator não terminou")
	}
}

func TestActorProcessesInOrderAndStops(t *testing.T) {
	out, reg := newOutbox(), &registry{}
	a := room.Start(context.Background(), actorOptions(out, reg), nil)
	a.Push(room.Join{UserID: "u0", DisplayName: "Ana"})
	a.Push(room.Join{UserID: "u1", DisplayName: "Bia"})
	a.Push(room.Stop{})
	waitDone(t, a)
	if out.count("u1", protocol.MsgRoomState) == 0 {
		t.Fatal("mensagens anteriores ao Stop devem ser processadas")
	}
	if a.Push(room.Wake{}) {
		t.Fatal("Push depois do fim deve devolver false")
	}
	if reg.destroyed != 0 {
		t.Fatal("Stop não destrói a sala (o lobby cuida do desligamento)")
	}
}

func TestActorAnswersJoinsQueuedAfterDestruction(t *testing.T) {
	out, reg := newOutbox(), &registry{}
	gate := &gatedOutbox{inner: out, gate: make(chan struct{})}
	a := room.Start(context.Background(), actorOptions(gate, reg), nil)
	// O primeiro envio (room_state do u0) segura o ator até as outras mensagens estarem na fila.
	a.Push(room.Join{UserID: "u0", DisplayName: "Ana"})
	a.Push(room.Command{UserID: "u0", Message: protocol.ClientMessage{Type: protocol.TypeLeaveRoom, Seq: 1}})
	if !a.Push(room.Join{UserID: "u1", DisplayName: "Bia", Seq: 2}) {
		t.Fatal("o ator ainda não pode ter fechado")
	}
	close(gate.gate)
	waitDone(t, a)
	codes := out.errorCodes("u1")
	if len(codes) != 1 || codes[0] != protocol.CodeRoomNotFound || !reg.wasReleased("u1") || reg.destroyed != 1 {
		t.Fatalf("códigos %v, destruída %d", codes, reg.destroyed)
	}
}

// gatedOutbox bloqueia o primeiro envio até gate fechar.
type gatedOutbox struct {
	inner *outbox
	gate  chan struct{}
	once  sync.Once
}

func (g *gatedOutbox) Send(user string, m protocol.ServerMessage) {
	g.once.Do(func() { <-g.gate })
	g.inner.Send(user, m)
}
