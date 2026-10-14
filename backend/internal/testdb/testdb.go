// Package testdb sobe um Postgres efêmero (testcontainers) por pacote de teste e
// entrega a cada teste um banco próprio clonado de um template já migrado.
package testdb

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/db"
)

const templateName = "truco_template"

type harness struct {
	container *postgres.PostgresContainer
	admin     *pgxpool.Pool
	baseURL   *url.URL
	counter   atomic.Int64
}

var (
	shared   *harness
	setupErr error
)

// Run deve ser chamado pelo TestMain dos pacotes com testes de integração.
func Run(m *testing.M) {
	flag.Parse()
	os.Exit(run(m))
}

func run(m *testing.M) int {
	if testing.Short() {
		return m.Run()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	h, err := start(ctx)
	cancel()
	if err != nil {
		setupErr = err
		return m.Run()
	}
	shared = h
	defer h.stop()
	return m.Run()
}

func start(ctx context.Context) (*harness, error) {
	container, err := postgres.Run(ctx, "postgres:18",
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("truco"),
		postgres.WithPassword("truco"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, fmt.Errorf("subir contêiner: %w", err)
	}
	h := &harness{container: container}
	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		h.stop()
		return nil, err
	}
	if h.baseURL, err = url.Parse(connStr); err != nil {
		h.stop()
		return nil, err
	}
	if h.admin, err = db.Open(ctx, connStr); err != nil {
		h.stop()
		return nil, err
	}
	if _, err := h.admin.Exec(ctx, "CREATE DATABASE "+templateName); err != nil {
		h.stop()
		return nil, err
	}
	template, err := db.Open(ctx, h.urlFor(templateName))
	if err != nil {
		h.stop()
		return nil, err
	}
	err = db.Migrate(ctx, template)
	template.Close() // o template não pode ter conexões abertas ao ser clonado
	if err != nil {
		h.stop()
		return nil, fmt.Errorf("migrar template: %w", err)
	}
	return h, nil
}

func (h *harness) urlFor(name string) string {
	u := *h.baseURL
	u.Path = "/" + name
	return u.String()
}

func (h *harness) stop() {
	if h.admin != nil {
		h.admin.Close()
	}
	if h.container != nil {
		_ = testcontainers.TerminateContainer(h.container)
	}
}

// NewURL cria um banco migrado exclusivo do teste e devolve sua URL.
func NewURL(t testing.TB) string {
	t.Helper()
	if testing.Short() {
		t.Skip("teste de integração: pulado com -short")
	}
	if shared == nil {
		msg := fmt.Sprintf("teste de integração: Postgres indisponível (%v). O Docker está rodando?", setupErr)
		if os.Getenv("TRUCO_REQUIRE_DB") == "1" {
			t.Fatal(msg)
		}
		t.Skip(msg)
	}
	name := fmt.Sprintf("t_%d", shared.counter.Add(1))
	ctx := context.Background()
	if _, err := shared.admin.Exec(ctx, "CREATE DATABASE "+name+" TEMPLATE "+templateName); err != nil {
		t.Fatalf("criar banco de teste: %v", err)
	}
	t.Cleanup(func() {
		_, _ = shared.admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})
	return shared.urlFor(name)
}

// New cria um banco migrado exclusivo do teste e devolve um pool para ele.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	pool, err := db.Open(context.Background(), NewURL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// CreateUser insere um usuário verificado e devolve seu id.
func CreateUser(t testing.TB, pool *pgxpool.Pool, username string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (username, email, display_name, password_hash, email_verified_at)
		 VALUES ($1, $1 || '@empresa.com', $1, 'x', now()) RETURNING id::text`, username).Scan(&id)
	if err != nil {
		t.Fatalf("criar usuário %s: %v", username, err)
	}
	return id
}
