// Comando server sobe a API do Truco e oferece tarefas administrativas.
//
// Uso: server [serve|migrate|promote-admin <email|username>|demote-admin <email|username>|recompute-stats]
//
// O CLI é o único caminho para criar ou remover administradores: nenhuma rota HTTP altera is_admin.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/admin"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/config"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/db"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/httpapi"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/lobby"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/mail"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/room"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/transport/ws"
)

// onListen é um gancho de teste chamado com o endereço real em que o servidor ouve.
var onListen = func(addr string) {}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string, stdout io.Writer) error {
	command := "serve"
	if len(args) > 0 {
		command, args = args[0], args[1:]
	}
	switch command {
	case "serve", "migrate", "promote-admin", "demote-admin", "recompute-stats":
	default:
		return fmt.Errorf("subcomando desconhecido %q (use serve, migrate, promote-admin, demote-admin ou recompute-stats)", command)
	}
	if command == "promote-admin" && len(args) != 1 {
		return errors.New("uso: server promote-admin <email|username>")
	}
	if command == "demote-admin" && len(args) != 1 {
		return errors.New("uso: server demote-admin <email|username>")
	}
	cfg, err := config.Load(getenv)
	if err != nil {
		return fmt.Errorf("configuração inválida:\n%w", err)
	}
	logger := newLogger(cfg, stdout)
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch command {
	case "migrate":
		return db.Migrate(ctx, pool)
	case "promote-admin":
		if err := admin.PromoteAdmin(ctx, pool, args[0], time.Now()); err != nil {
			return fmt.Errorf("promover %q: %w", args[0], err)
		}
		fmt.Fprintf(stdout, "%s agora é administrador\n", args[0])
		return nil
	case "demote-admin":
		err := admin.DemoteAdmin(ctx, pool, args[0], time.Now())
		if errors.Is(err, admin.ErrNotAdmin) {
			fmt.Fprintf(stdout, "%s não é administrador\n", args[0])
			return nil
		}
		if err != nil {
			return fmt.Errorf("rebaixar %q: %w", args[0], err)
		}
		fmt.Fprintf(stdout, "%s não é mais administrador\n", args[0])
		return nil
	case "recompute-stats":
		skipped, err := match.NewStore(pool, nil).RecomputeStats(ctx)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "estatísticas recalculadas; partidas puladas: %d\n", len(skipped))
		for _, id := range skipped {
			fmt.Fprintf(stdout, "  pulada: %s\n", id)
		}
		return nil
	}
	if cfg.MigrateOnStart {
		if err := db.Migrate(ctx, pool); err != nil {
			return err
		}
	}
	return serve(ctx, cfg, pool, logger)
}

func newLogger(cfg config.Config, w io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.LogFormat == "text" {
		return slog.New(slog.NewTextHandler(w, opts))
	}
	return slog.New(slog.NewJSONHandler(w, opts))
}

func newMailer(cfg config.Config, logger *slog.Logger) mail.Mailer {
	if cfg.MailDriver == config.MailLog {
		return mail.NewLog(logger)
	}
	return mail.NewResend(cfg.ResendAPIKey, cfg.MailFrom)
}

func serve(ctx context.Context, cfg config.Config, pool *pgxpool.Pool, logger *slog.Logger) error {
	authSvc := auth.NewService(auth.Options{
		Pool: pool, Mailer: newMailer(cfg, logger), Hasher: auth.NewHasher(auth.DefaultParams, 4),
		BaseURL: cfg.AppBaseURL, Cookies: auth.CookieConfig{Secure: cfg.CookieSecure}, Logger: logger,
	})
	if err := authSvc.BootstrapDomains(ctx, cfg.AllowedEmailDomains); err != nil {
		return err
	}
	store := match.NewStore(pool, nil)
	recovered, err := store.Recover(ctx, logger)
	if err != nil {
		return fmt.Errorf("recuperar partidas: %w", err)
	}
	lob := lobby.New(context.Background(), lobby.Options{Timing: room.Timing(cfg.Game), MaxRooms: cfg.MaxRooms,
		Store: lobby.NewMatchStore(store), Clock: clock.Real{}, Rand: rand.Reader, Log: logger})
	if err := lob.Restore(recovered); err != nil {
		return fmt.Errorf("restaurar salas: %w", err)
	}
	logger.Info("partidas retomadas", "count", len(recovered))
	api := httpapi.New(httpapi.Deps{
		Config: cfg, Auth: authSvc, Stats: stats.NewReader(pool, cfg.RankingMinMatches, nil), Matches: store,
		Admin: admin.NewService(admin.Options{Pool: pool, Auth: authSvc, Aborter: lob, Logger: logger}),
		WS:    &ws.Handler{Auth: authSvc, Lobby: lob, Origin: cfg.AppOrigin, Limits: ws.DefaultLimits, Log: logger},
		Pool:  pool, Logger: logger,
	})
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("ouvir em %s: %w", cfg.HTTPAddr, err)
	}
	// Os timeouts valem para a API; o handler /ws zera os deadlines da conexão longa.
	srv := &http.Server{
		Handler: api.Handler(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second,
		WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second,
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
	maintCtx, stopMaint := context.WithCancel(context.Background())
	var maint sync.WaitGroup
	maint.Go(func() { api.RunMaintenance(maintCtx) })
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(listener) }()
	logger.Info("servidor ouvindo", "addr", listener.Addr().String())
	onListen(listener.Addr().String())

	select {
	case err := <-errCh:
		stopMaint()
		maint.Wait()
		return err
	case <-ctx.Done():
	}
	logger.Info("desligando o servidor")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	return shutdown(shutdownCtx, logger, lob, srv, errCh, stopMaint, &maint, authSvc)
}

// shutdown segue a Spec 3 §9.2: salas e conexões, HTTP, manutenção e e-mails em envio.
func shutdown(ctx context.Context, logger *slog.Logger, lob *lobby.Lobby, srv *http.Server, errCh <-chan error,
	stopMaint context.CancelFunc, maint *sync.WaitGroup, authSvc *auth.Service) error {
	var errs []error
	if err := lob.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("salas: %w", err))
	}
	if err := srv.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("http: %w", err))
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		errs = append(errs, err)
	}
	stopMaint()
	maint.Wait()
	if err := authSvc.Wait(ctx); err != nil {
		logger.Warn("e-mails ainda em envio no desligamento", "err", err)
	}
	return errors.Join(errs...)
}
