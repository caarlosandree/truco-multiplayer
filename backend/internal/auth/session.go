package auth

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db/dbgen"
)

const (
	SessionIdleTTL       = 30 * 24 * time.Hour
	SessionAbsoluteTTL   = 90 * 24 * time.Hour
	SessionTouchInterval = 24 * time.Hour
	maxUserAgent         = 256
)

type Principal struct {
	UserID      string
	SessionID   string
	Username    string
	Email       string
	DisplayName string
	IsAdmin     bool
}

type SessionMeta struct {
	UserAgent string
	IP        string
}

func (s *Service) createSession(ctx context.Context, q *dbgen.Queries, userID string, meta SessionMeta) (string, error) {
	now := s.now()
	token, hash := NewToken()
	userAgent := meta.UserAgent
	if len(userAgent) > maxUserAgent {
		userAgent = strings.ToValidUTF8(userAgent[:maxUserAgent], "")
	}
	// A coluna é inet: IP inválido faria o INSERT falhar, então grava vazio (NULL).
	ip := ""
	if parsed, err := netip.ParseAddr(meta.IP); err == nil {
		ip = parsed.WithZone("").Unmap().String()
	}
	_, err := q.CreateSession(ctx, dbgen.CreateSessionParams{
		UserID: userID, TokenHash: hash, Now: now, ExpiresAt: now.Add(SessionIdleTTL),
		AbsoluteExpiresAt: now.Add(SessionAbsoluteTTL), UserAgent: userAgent, Ip: ip,
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// Authenticate valida o token e renova a janela deslizante no máximo uma vez a cada 24h.
func (s *Service) Authenticate(ctx context.Context, token string) (Principal, error) {
	if token == "" {
		return Principal{}, ErrUnauthenticated
	}
	now := s.now()
	row, err := s.q.GetSessionPrincipal(ctx, dbgen.GetSessionPrincipalParams{TokenHash: HashToken(token), Now: now})
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, err
	}
	if now.Sub(row.LastSeenAt) > SessionTouchInterval {
		if err := s.q.TouchSession(ctx, dbgen.TouchSessionParams{Now: now, ExpiresAt: now.Add(SessionIdleTTL), ID: row.SessionID}); err != nil {
			return Principal{}, err
		}
	}
	return Principal{UserID: row.UserID, SessionID: row.SessionID, Username: row.Username, Email: deref(row.Email),
		DisplayName: row.DisplayName, IsAdmin: row.IsAdmin}, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.q.DeleteSessionByTokenHash(ctx, HashToken(token))
}

func (s *Service) LogoutAll(ctx context.Context, userID string) error {
	return s.q.DeleteUserSessions(ctx, userID)
}

// CleanupExpired remove sessões expiradas e tokens vencidos ou usados há mais de 7 dias.
func (s *Service) CleanupExpired(ctx context.Context) (sessions, tokens int64, err error) {
	now := s.now()
	if sessions, err = s.q.DeleteExpiredSessions(ctx, now); err != nil {
		return 0, 0, err
	}
	tokens, err = s.q.DeleteStaleTokens(ctx, dbgen.DeleteStaleTokensParams{Now: now, UsedBefore: now.Add(-7 * 24 * time.Hour)})
	return sessions, tokens, err
}

// SessionFromRequest autentica pelo cookie. Também é usado no handshake do WebSocket (Spec 3).
func (s *Service) SessionFromRequest(r *http.Request) (Principal, error) {
	return s.Authenticate(r.Context(), s.cookies.Read(r))
}

type CookieConfig struct {
	Secure bool
}

func (c CookieConfig) Name() string {
	if c.Secure {
		return "__Host-truco_session"
	}
	return "truco_session"
}

func (c CookieConfig) Set(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: c.Name(), Value: token, Path: "/", MaxAge: int(SessionAbsoluteTTL.Seconds()),
		HttpOnly: true, Secure: c.Secure, SameSite: http.SameSiteLaxMode})
}

func (c CookieConfig) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: c.Name(), Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: c.Secure, SameSite: http.SameSiteLaxMode})
}

func (c CookieConfig) Read(r *http.Request) string {
	cookie, err := r.Cookie(c.Name())
	if err != nil {
		return ""
	}
	return cookie.Value
}

// CheckOrigin aceita a requisição se o Origin for o do app ou, sem Origin,
// se o navegador declarar Sec-Fetch-Site: same-origin.
func CheckOrigin(r *http.Request, allowedOrigin string) bool {
	if origin := r.Header.Get("Origin"); origin != "" {
		return origin == allowedOrigin
	}
	return r.Header.Get("Sec-Fetch-Site") == "same-origin"
}
