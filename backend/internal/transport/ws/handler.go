// Package ws expõe o lobby por WebSocket em GET /ws: autentica o upgrade pelo cookie de sessão,
// mantém uma conexão por usuário e aplica limites de tamanho, taxa e validade da sessão.
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/lobby"
)

// Authenticator é o que o handler usa de auth.Service.
type Authenticator interface {
	SessionFromRequest(r *http.Request) (auth.Principal, error)
	Authenticate(ctx context.Context, token string) (auth.Principal, error)
	Cookies() auth.CookieConfig
}

type Limits struct {
	MaxMessage int     // bytes; acima disso fecha com 4008
	RatePerSec float64 // mensagens por segundo
	Burst      int
	Queue      int // fila de saída; cheia derruba o cliente lento
	PingEvery  time.Duration
	PongWait   time.Duration
	WriteWait  time.Duration
	Revalidate time.Duration // intervalo de revalidação da sessão
}

var DefaultLimits = Limits{MaxMessage: 4096, RatePerSec: 20, Burst: 40, Queue: 64,
	PingEvery: 20 * time.Second, PongWait: 10 * time.Second, WriteWait: 5 * time.Second, Revalidate: 5 * time.Minute}

// hardReadLimit é o teto da biblioteca: acima dele ela mesma fecha com 1009.
const hardReadLimit = 64 << 10

const (
	codeReplaced       websocket.StatusCode = 4001
	codeSessionInvalid websocket.StatusCode = 4003
	codePolicy         websocket.StatusCode = 4008
)

type Handler struct {
	Auth   Authenticator
	Lobby  *lobby.Lobby
	Origin string
	Limits Limits
	Log    *slog.Logger
}

func (h *Handler) log() *slog.Logger {
	if h.Log == nil {
		return slog.Default()
	}
	return h.Log
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p, err := h.Auth.SessionFromRequest(r)
	if errors.Is(err, auth.ErrUnauthenticated) {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "faça login para jogar")
		return
	}
	if err != nil {
		h.log().ErrorContext(r.Context(), "falha ao autenticar o WebSocket", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "erro interno")
		return
	}
	if !auth.CheckOrigin(r, h.Origin) {
		writeError(w, http.StatusForbidden, "forbidden_origin", "origem não permitida")
		return
	}
	// Os timeouts do http.Server valem para a API; a conexão longa usa os próprios limites.
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})
	// A origem já foi conferida contra APP_ORIGIN, o que também funciona atrás de proxy.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	newClient(h, conn, p, h.Auth.Cookies().Read(r)).serve()
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}
