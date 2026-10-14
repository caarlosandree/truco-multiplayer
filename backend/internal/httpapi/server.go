// Package httpapi expõe a API JSON de contas, estatísticas e administração.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/admin"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/config"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/ratelimit"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
)

type Deps struct {
	Config  config.Config
	Auth    *auth.Service
	Stats   *stats.Reader
	Matches *match.Store
	Admin   *admin.Service
	Pool    *pgxpool.Pool
	Logger  *slog.Logger
	Now     func() time.Time
}

type Server struct {
	deps          Deps
	log           *slog.Logger
	mux           *http.ServeMux
	handler       http.Handler
	static        http.Handler // build do frontend; nil sem STATIC_DIR
	loginIP       *ratelimit.Limiter
	pwLockout     *ratelimit.Lockout // falhas de senha no login ("login:") e na troca de senha ("pw:")
	registerIP    *ratelimit.Limiter
	recoveryIP    *ratelimit.Limiter
	recoveryEmail *ratelimit.Limiter
	tokenIP       *ratelimit.Limiter
}

func New(d Deps) *Server {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	s := &Server{
		deps: d, log: d.Logger, mux: http.NewServeMux(),
		loginIP:       ratelimit.NewLimiter(10, time.Minute, d.Now),
		pwLockout:     ratelimit.NewLockout(5, 15*time.Minute, 15*time.Minute, d.Now),
		registerIP:    ratelimit.NewLimiter(5, time.Hour, d.Now),
		recoveryIP:    ratelimit.NewLimiter(5, time.Hour, d.Now),
		recoveryEmail: ratelimit.NewLimiter(3, time.Hour, d.Now),
		tokenIP:       ratelimit.NewLimiter(20, time.Hour, d.Now),
	}
	s.registerAuthRoutes()
	s.registerReadRoutes()
	s.registerAdminRoutes()
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	if d.Config.StaticDir != "" {
		s.static = spaHandler(d.Config.StaticDir)
	}
	// Um único padrão "/" evita o conflito do ServeMux entre "GET /" (SPA) e "/api/" (404 JSON).
	s.mux.HandleFunc("/", s.handleFallback)
	s.handler = s.requestID(s.recoverer(s.accessLog(securityHeaders(s.session(s.originCheck(s.mux))))))
	return s
}

func (s *Server) Handler() http.Handler { return s.handler }

// handleFallback responde o que nenhuma rota tratou: SPA para GET fora de /api, 404 JSON no resto.
func (s *Server) handleFallback(w http.ResponseWriter, r *http.Request) {
	isRead := r.Method == http.MethodGet || r.Method == http.MethodHead
	if s.static != nil && isRead && !isAPI(r) {
		s.static.ServeHTTP(w, r)
		return
	}
	s.writeError(w, r, errNotFoundRoute)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.deps.Pool.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "indisponível"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// RunMaintenance roda até ctx terminar: varre os rate limits e limpa sessões e tokens vencidos.
func (s *Server) RunMaintenance(ctx context.Context) {
	sweep := time.NewTicker(5 * time.Minute)
	cleanup := time.NewTicker(time.Hour)
	defer sweep.Stop()
	defer cleanup.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-sweep.C:
			for _, l := range []*ratelimit.Limiter{s.loginIP, s.registerIP, s.recoveryIP, s.recoveryEmail, s.tokenIP} {
				l.Sweep()
			}
			s.pwLockout.Sweep()
		case <-cleanup.C:
			sessions, tokens, err := s.deps.Auth.CleanupExpired(ctx)
			if err != nil {
				s.log.ErrorContext(ctx, "falha na limpeza de sessões", "err", err)
				continue
			}
			s.log.InfoContext(ctx, "limpeza concluída", "sessions", sessions, "tokens", tokens)
		}
	}
}
