package room_test

import (
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
)

func TestSignalsBroadcastAndRateLimit(t *testing.T) {
	h := newHarness(t)
	h.startMatch()
	h.cmd("u1", protocol.TypeSignal, protocol.SignalPayload{ID: protocol.SignalTruco})
	for _, u := range users {
		if s := last[protocol.SignalBroadcastPayload](t, h.out, u, protocol.MsgSignal); s.Seat != 1 || s.ID != protocol.SignalTruco {
			t.Fatalf("%s: sinal = %+v", u, s)
		}
	}
	h.cmd("u1", protocol.TypeSignal, protocol.SignalPayload{ID: protocol.SignalBoa})
	if lastCode(t, h.out, "u1") != protocol.CodeRateLimited {
		t.Fatal("dois sinais em menos de 3s")
	}
	h.advance(3 * time.Second)
	h.cmd("u1", protocol.TypeSignal, protocol.SignalPayload{ID: protocol.SignalBoa})
	if h.out.count("u0", protocol.MsgSignal) != 2 {
		t.Fatal("depois de 3s o sinal volta a valer")
	}
	h.cmd("u1", protocol.TypeSignal, map[string]string{"id": "xingar"})
	if lastCode(t, h.out, "u1") != protocol.CodeBadRequest {
		t.Fatal("sinal fora da lista é bad_request")
	}
}

func TestSignalOutsideMatchIsBadRequest(t *testing.T) {
	h := newHarness(t)
	h.join(0)
	h.cmd("u0", protocol.TypeSignal, protocol.SignalPayload{ID: protocol.SignalBoa})
	if lastCode(t, h.out, "u0") != protocol.CodeBadRequest {
		t.Fatal("sinais só durante a partida")
	}
}
