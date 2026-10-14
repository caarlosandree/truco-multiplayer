package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/netip"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/ratelimit"
)

type ctxKey int

const (
	requestIDKey ctxKey = iota
	principalKey
)

func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func principalFrom(ctx context.Context) (auth.Principal, bool) {
	p, ok := ctx.Value(principalKey).(auth.Principal)
	return p, ok
}

func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := make([]byte, 8)
		_, _ = rand.Read(raw)
		id := hex.EncodeToString(raw)
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec)
			}
			s.log.ErrorContext(r.Context(), "panic na requisição", "request_id", requestIDFrom(r.Context()),
				"panic", fmt.Sprint(rec), "stack", string(debug.Stack()))
			s.writeError(w, r, errInternal)
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// accessLog registra método, caminho, status e duração. Nunca registra corpo nem cookies.
func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.InfoContext(r.Context(), "http", "method", r.Method, "path", r.URL.Path, "status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(), "request_id", requestIDFrom(r.Context()))
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		if isAPI(r) {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func isAPI(r *http.Request) bool { return strings.HasPrefix(r.URL.Path, "/api/") }

func isMutating(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

// session coloca o Principal no contexto. Cookie inválido é limpo e a requisição segue anônima.
func (s *Server) session(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookies := s.deps.Auth.Cookies()
		token := cookies.Read(r)
		if !isAPI(r) || token == "" {
			next.ServeHTTP(w, r)
			return
		}
		p, err := s.deps.Auth.Authenticate(r.Context(), token)
		switch {
		case err == nil:
			r = r.WithContext(context.WithValue(r.Context(), principalKey, p))
		case errors.Is(err, auth.ErrUnauthenticated):
			cookies.Clear(w)
		default:
			s.writeError(w, r, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAPI(r) && isMutating(r.Method) && !auth.CheckOrigin(r, s.deps.Config.AppOrigin) {
			s.writeError(w, r, errForbiddenOrigin)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type authedHandler func(http.ResponseWriter, *http.Request, auth.Principal)

func (s *Server) requireAuth(h authedHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := principalFrom(r.Context())
		if !ok {
			s.writeError(w, r, auth.ErrUnauthenticated)
			return
		}
		h(w, r, p)
	})
}

func (s *Server) requireAdmin(h authedHandler) http.Handler {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request, p auth.Principal) {
		if !p.IsAdmin {
			s.writeError(w, r, errForbidden)
			return
		}
		h(w, r, p)
	})
}

// clientIP usa RemoteAddr; com TRUST_PROXY, o último endereço válido de X-Forwarded-For.
func (s *Server) clientIP(r *http.Request) string {
	if s.deps.Config.TrustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if addr, err := netip.ParseAddr(strings.TrimSpace(parts[len(parts)-1])); err == nil {
				return addr.String()
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	// Só devolve IP válido: a coluna de sessão usa NULLIF(ip, '')::inet.
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return ""
	}
	return addr.String()
}

func (s *Server) sessionMeta(r *http.Request) auth.SessionMeta {
	return auth.SessionMeta{UserAgent: r.UserAgent(), IP: s.clientIP(r)}
}

func (s *Server) writeTooMany(w http.ResponseWriter, r *http.Request, retry time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
	s.writeError(w, r, errTooMany)
}

func (s *Server) allow(w http.ResponseWriter, r *http.Request, l *ratelimit.Limiter, key string) bool {
	ok, retry := l.Allow(key)
	if !ok {
		s.writeTooMany(w, r, retry)
	}
	return ok
}
