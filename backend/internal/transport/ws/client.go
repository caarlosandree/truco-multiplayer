package ws

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/time/rate"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/lobby"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/protocol"
)

// client é uma conexão de usuário: a leitura roda numa goroutine, a escrita em outra
// (a de serve), e o fim é sinalizado por done com o código de fechamento escolhido.
type client struct {
	h       *Handler
	conn    *websocket.Conn
	info    protocol.UserInfo
	token   string
	out     chan protocol.ServerMessage
	limiter *rate.Limiter
	done    chan struct{}
	once    sync.Once
	code    websocket.StatusCode // 0 = o par sumiu; fecha sem handshake
	reason  string
}

func newClient(h *Handler, conn *websocket.Conn, p auth.Principal, token string) *client {
	return &client{h: h, conn: conn, token: token,
		info:    protocol.UserInfo{ID: p.UserID, Username: p.Username, DisplayName: p.DisplayName},
		out:     make(chan protocol.ServerMessage, h.Limits.Queue),
		limiter: rate.NewLimiter(rate.Limit(h.Limits.RatePerSec), h.Limits.Burst),
		done:    make(chan struct{})}
}

func (c *client) UserID() string { return c.info.ID }

// Send enfileira sem bloquear; fila cheia derruba o cliente lento, que reconecta e recebe o snapshot.
func (c *client) Send(m protocol.ServerMessage) {
	select {
	case <-c.done:
	case c.out <- m:
	default:
		c.closeWith(websocket.StatusTryAgainLater, "fila de saída cheia")
	}
}

func (c *client) Close(reason lobby.CloseReason) {
	if reason == lobby.CloseReplaced {
		c.closeWith(codeReplaced, "replaced")
		return
	}
	if reason == lobby.CloseRevoked {
		c.closeWith(codeSessionInvalid, "session_invalid")
		return
	}
	c.closeWith(websocket.StatusGoingAway, "going_away")
}

func (c *client) closeWith(code websocket.StatusCode, reason string) {
	c.once.Do(func() {
		c.code, c.reason = code, reason
		close(c.done)
	})
}

func (c *client) serve() {
	c.conn.SetReadLimit(hardReadLimit)
	if !c.h.Lobby.Connect(c, c.info) {
		_ = c.conn.Close(websocket.StatusGoingAway, "going_away")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.h.log().Debug("conexão WebSocket aberta", "user_id", c.info.ID)
	go c.readLoop(ctx)
	c.writeLoop(ctx)
	c.h.Lobby.Disconnect(c)
	c.h.log().Debug("conexão WebSocket fechada", "user_id", c.info.ID, "code", int(c.code))
}

func (c *client) readLoop(ctx context.Context) {
	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			c.closeWith(0, "")
			return
		}
		if len(data) > c.h.Limits.MaxMessage {
			c.policyClose("mensagem grande demais")
			return
		}
		if !c.limiter.Allow() {
			c.policyClose("mensagens demais")
			return
		}
		var msg protocol.ClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			c.Send(protocol.ErrorMsg(protocol.CodeBadRequest, 0))
			continue
		}
		c.h.Lobby.Dispatch(c, msg)
	}
}

func (c *client) policyClose(reason string) {
	c.h.log().Warn("conexão fechada por política", "user_id", c.info.ID, "reason", reason)
	c.closeWith(codePolicy, reason)
}

func (c *client) writeLoop(ctx context.Context) {
	ping := time.NewTicker(c.h.Limits.PingEvery)
	defer ping.Stop()
	revalidate := time.NewTicker(c.h.Limits.Revalidate)
	defer revalidate.Stop()
	for {
		select {
		case <-c.done:
			if c.code == 0 {
				_ = c.conn.CloseNow()
			} else {
				_ = c.conn.Close(c.code, c.reason)
			}
			return
		case m := <-c.out:
			if err := c.write(ctx, m); err != nil {
				c.closeWith(0, "")
			}
		case <-ping.C:
			pctx, cancel := context.WithTimeout(ctx, c.h.Limits.PongWait)
			err := c.conn.Ping(pctx)
			cancel()
			if err != nil {
				c.closeWith(0, "")
			}
		case <-revalidate.C:
			actx, cancel := context.WithTimeout(ctx, revalidateTimeout)
			_, err := c.h.Auth.Authenticate(actx, c.token)
			cancel()
			if errors.Is(err, auth.ErrUnauthenticated) {
				c.closeWith(codeSessionInvalid, "session_invalid")
			}
		}
	}
}

// revalidateTimeout limita a consulta de sessão para que um banco lento não trave o laço de escrita.
const revalidateTimeout = 5 * time.Second

func (c *client) write(ctx context.Context, m protocol.ServerMessage) error {
	data, err := json.Marshal(m)
	if err != nil {
		c.h.log().Error("falha ao serializar mensagem", "type", m.Type, "err", err)
		return nil
	}
	wctx, cancel := context.WithTimeout(ctx, c.h.Limits.WriteWait)
	defer cancel()
	return c.conn.Write(wctx, websocket.MessageText, data)
}
