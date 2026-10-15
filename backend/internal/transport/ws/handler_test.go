package ws_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/lobby"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/transport/ws"
)

const origin = "http://truco.test"

type fakeAuth struct {
	mu       sync.Mutex
	sessions map[string]auth.Principal
}

func (f *fakeAuth) Cookies() auth.CookieConfig { return auth.CookieConfig{Secure: false} }

func (f *fakeAuth) Authenticate(_ context.Context, token string) (auth.Principal, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.sessions[token]
	if !ok {
		return auth.Principal{}, auth.ErrUnauthenticated
	}
	return p, nil
}

func (f *fakeAuth) SessionFromRequest(r *http.Request) (auth.Principal, error) {
	return f.Authenticate(r.Context(), f.Cookies().Read(r))
}

func (f *fakeAuth) revoke(token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.sessions, token)
}

type nopStore struct{}

func (nopStore) Create(context.Context, uint64, engine.Seat, [4]replay.PlayerSeat) (string, error) {
	return "m1", nil
}
func (nopStore) AppendAction(context.Context, string, int, engine.Action, replay.Source) error {
	return nil
}
func (nopStore) Finish(context.Context, string) error          { return nil }
func (nopStore) Abandon(context.Context, string, string) error { return nil }

type env struct {
	srv  *httptest.Server
	auth *fakeAuth
}

func newEnv(t *testing.T, tweak ...func(*ws.Limits)) env {
	a := &fakeAuth{sessions: map[string]auth.Principal{
		"tok-ana": {UserID: "u0", Username: "ana", DisplayName: "Ana"},
		"tok-bia": {UserID: "u1", Username: "bia", DisplayName: "Bia"},
	}}
	discard := slog.New(slog.NewTextHandler(io.Discard, nil))
	l := lobby.New(context.Background(), lobby.Options{Timing: room.Timing{Play: time.Minute, Vote: time.Minute,
		Choice: time.Minute, Burn: time.Minute, Absent: time.Minute, AbsenceGrace: time.Hour, AbsenceBudget: time.Hour,
		WaitingDisconnect: time.Hour}, MaxRooms: 5, Store: nopStore{}, Clock: clock.NewManual(time.Now()),
		Rand: rand.NewChaCha8([32]byte{3}), Log: discard})
	limits := ws.DefaultLimits
	for _, f := range tweak {
		f(&limits)
	}
	srv := httptest.NewServer(&ws.Handler{Auth: a, Lobby: l, Origin: origin, Limits: limits, Log: discard})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = l.Shutdown(ctx)
		srv.Close()
	})
	return env{srv: srv, auth: a}
}

func (e env) dial(token, from string) (*websocket.Conn, *http.Response, error) {
	header := http.Header{}
	if from != "" {
		header.Set("Origin", from)
	}
	if token != "" {
		header.Set("Cookie", "truco_session="+token)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return websocket.Dial(ctx, "ws"+strings.TrimPrefix(e.srv.URL, "http"), &websocket.DialOptions{HTTPHeader: header})
}

func (e env) connect(t *testing.T, token string) *websocket.Conn {
	t.Helper()
	c, _, err := e.dial(token, origin)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

type inbound struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func readType(t *testing.T, c *websocket.Conn, typ string) inbound {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("esperando %s: %v", typ, err)
		}
		var m inbound
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		if m.Type == typ {
			return m
		}
	}
}

func write(t *testing.T, c *websocket.Conn, data string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, []byte(data)); err != nil {
		t.Fatal(err)
	}
}

func closeStatus(t *testing.T, c *websocket.Conn) websocket.StatusCode {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		if _, _, err := c.Read(ctx); err != nil {
			return websocket.CloseStatus(err)
		}
	}
}

func TestRejectsMissingSessionAndForeignOrigin(t *testing.T) {
	e := newEnv(t)
	if _, resp, err := e.dial("", origin); err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("sem sessão: %v %v", resp, err)
	}
	if _, resp, err := e.dial("tok-ana", "http://evil.test"); err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("origem estranha: %v %v", resp, err)
	}
}

func TestHelloOnConnect(t *testing.T) {
	e := newEnv(t)
	c := e.connect(t, "tok-ana")
	var hello protocol.HelloPayload
	if err := json.Unmarshal(readType(t, c, "hello").Payload, &hello); err != nil || hello.User.ID != "u0" || hello.User.DisplayName != "Ana" {
		t.Fatalf("hello = %+v (%v)", hello, err)
	}
	readType(t, c, "lobby_snapshot")
}

func TestSecondConnectionClosesFirstWith4001(t *testing.T) {
	e := newEnv(t)
	first := e.connect(t, "tok-ana")
	readType(t, first, "hello")
	second := e.connect(t, "tok-ana")
	readType(t, second, "hello")
	if code := closeStatus(t, first); code != 4001 {
		t.Fatalf("código = %d", code)
	}
}

func TestOversizedMessageClosesWith4008(t *testing.T) {
	e := newEnv(t)
	c := e.connect(t, "tok-ana")
	readType(t, c, "hello")
	write(t, c, `{"type":"leave_room","payload":"`+string(bytes.Repeat([]byte("a"), 5000))+`"}`)
	if code := closeStatus(t, c); code != 4008 {
		t.Fatalf("código = %d", code)
	}
}

func TestFloodClosesWith4008(t *testing.T) {
	e := newEnv(t, func(l *ws.Limits) { l.RatePerSec, l.Burst = 1, 2 })
	c := e.connect(t, "tok-ana")
	readType(t, c, "hello")
	for i := range 4 {
		write(t, c, fmt.Sprintf(`{"type":"leave_room","seq":%d}`, i))
	}
	if code := closeStatus(t, c); code != 4008 {
		t.Fatalf("código = %d", code)
	}
}

func TestMalformedMessageGetsBadRequestAndKeepsConnection(t *testing.T) {
	e := newEnv(t)
	c := e.connect(t, "tok-ana")
	readType(t, c, "hello")
	write(t, c, `{`)
	var p protocol.ErrorPayload
	if err := json.Unmarshal(readType(t, c, "error").Payload, &p); err != nil || p.Code != protocol.CodeBadRequest {
		t.Fatalf("erro = %+v", p)
	}
	write(t, c, `{"type":"create_room","seq":2,"payload":{"visibility":"public"}}`)
	readType(t, c, "room_state")
}

func TestRevalidationClosesRevokedSession(t *testing.T) {
	e := newEnv(t, func(l *ws.Limits) { l.Revalidate = 50 * time.Millisecond })
	c := e.connect(t, "tok-ana")
	readType(t, c, "hello")
	e.auth.revoke("tok-ana")
	if code := closeStatus(t, c); code != 4003 {
		t.Fatalf("código = %d", code)
	}
}
