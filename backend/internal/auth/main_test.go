package auth

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/mail"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testclock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testdb"
)

func TestMain(m *testing.M) { testdb.Run(m) }

var t0 = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

type fixture struct {
	svc   *Service
	pool  *pgxpool.Pool
	mail  *mail.Fake
	clock *testclock.Clock
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := testdb.New(t)
	f := fixture{pool: pool, mail: &mail.Fake{}, clock: testclock.New(t0)}
	f.svc = NewService(Options{
		Pool: pool, Mailer: f.mail, Hasher: NewHasher(TestingParams, 4), Now: f.clock.Now,
		BaseURL: "https://truco.test", Cookies: CookieConfig{Secure: true},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Background: func(fn func()) { fn() },
	})
	if err := f.svc.BootstrapDomains(context.Background(), []string{"empresa.com"}); err != nil {
		t.Fatal(err)
	}
	return f
}

// newUser cria um usuário verificado com a senha informada (sem passar pelo fluxo de cadastro).
func (f fixture) newUser(t *testing.T, username, password string) string {
	t.Helper()
	hash, err := f.svc.hasher.Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	var id string
	err = f.pool.QueryRow(context.Background(),
		`INSERT INTO users (username, email, display_name, password_hash, email_verified_at)
		 VALUES ($1, $1 || '@empresa.com', initcap($1), $2, now()) RETURNING id::text`, username, hash).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
