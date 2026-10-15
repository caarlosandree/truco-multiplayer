package auth

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/db/dbgen"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/mail"
)

type Options struct {
	Pool       *pgxpool.Pool
	Mailer     mail.Mailer
	Hasher     *Hasher
	Now        func() time.Time
	BaseURL    string
	Cookies    CookieConfig
	Logger     *slog.Logger
	Background func(func())
}

type Service struct {
	pool       *pgxpool.Pool
	q          *dbgen.Queries
	mailer     mail.Mailer
	hasher     *Hasher
	now        func() time.Time
	baseURL    string
	cookies    CookieConfig
	log        *slog.Logger
	background func(func())
	pending    sync.WaitGroup // e-mails em envio
}

func NewService(o Options) *Service {
	s := &Service{pool: o.Pool, q: dbgen.New(o.Pool), mailer: o.Mailer, hasher: o.Hasher, now: o.Now,
		baseURL: o.BaseURL, cookies: o.Cookies, log: o.Logger, background: o.Background}
	if s.now == nil {
		s.now = time.Now
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	if s.background == nil {
		s.background = func(fn func()) { go fn() }
	}
	return s
}

type User struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	IsAdmin     bool      `json:"isAdmin"`
	CreatedAt   time.Time `json:"createdAt"`
}

func userFromRow(u dbgen.User) User {
	return User{ID: u.ID, Username: u.Username, Email: u.Email, DisplayName: u.DisplayName, IsAdmin: u.IsAdmin, CreatedAt: u.CreatedAt}
}

func (s *Service) Cookies() CookieConfig { return s.cookies }

func (s *Service) GetUser(ctx context.Context, userID string) (User, error) {
	row, err := s.q.GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, err
	}
	return userFromRow(row), nil
}

// BootstrapDomains popula allowed_email_domains a partir da configuração, só se a tabela estiver vazia.
func (s *Service) BootstrapDomains(ctx context.Context, domains []string) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.LockDomains(ctx); err != nil {
			return err
		}
		count, err := q.CountDomains(ctx)
		if err != nil || count > 0 {
			return err
		}
		if len(domains) == 0 {
			return errors.New("auth: nenhum domínio de e-mail permitido; defina ALLOWED_EMAIL_DOMAINS")
		}
		for _, d := range domains {
			if err := q.InsertDomain(ctx, dbgen.InsertDomainParams{Domain: d, Now: s.now()}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) link(path string) string { return s.baseURL + path }

// sendLater envia fora da requisição: o tempo de resposta não revela se houve envio.
func (s *Service) sendLater(m mail.Message) {
	s.pending.Add(1)
	s.background(func() {
		defer s.pending.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.mailer.Send(ctx, m); err != nil {
			s.log.Error("falha ao enviar e-mail", "to", m.To, "subject", m.Subject, "err", err)
		}
	})
}

// Wait aguarda os e-mails em envio até ctx terminar; usado no desligamento gracioso.
func (s *Service) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		s.pending.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
