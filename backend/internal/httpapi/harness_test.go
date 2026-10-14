package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/admin"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/config"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/httpapi"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/mail"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testclock"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/testdb"
)

func TestMain(m *testing.M) { testdb.Run(m) }

const origin = "http://truco.test"

type harness struct {
	t      *testing.T
	srv    *httptest.Server
	mail   *mail.Fake
	clock  *testclock.Clock
	pool   *pgxpool.Pool
	store  *match.Store
	nextIP atomic.Int32
}

func newHarness(t *testing.T, mutate ...func(*httpapi.Deps)) *harness {
	t.Helper()
	pool := testdb.New(t)
	h := &harness{t: t, mail: &mail.Fake{}, clock: testclock.New(time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)), pool: pool}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// TrustProxy permite que cada cliente de teste tenha seu próprio IP via X-Forwarded-For.
	cfg := config.Config{AppBaseURL: origin, AppOrigin: origin, CookieSecure: false, TrustProxy: true, RankingMinMatches: 1}
	authSvc := auth.NewService(auth.Options{Pool: pool, Mailer: h.mail, Hasher: auth.NewHasher(auth.TestingParams, 4), Now: h.clock.Now,
		BaseURL: origin, Cookies: auth.CookieConfig{Secure: false}, Logger: logger, Background: func(fn func()) { fn() }})
	if err := authSvc.BootstrapDomains(context.Background(), []string{"empresa.com"}); err != nil {
		t.Fatal(err)
	}
	h.store = match.NewStore(pool, h.clock.Now)
	deps := httpapi.Deps{Config: cfg, Auth: authSvc, Stats: stats.NewReader(pool, 1), Matches: h.store,
		Admin: admin.NewService(admin.Options{Pool: pool, Auth: authSvc, Now: h.clock.Now, Logger: logger}),
		Pool:  pool, Logger: logger, Now: h.clock.Now}
	for _, m := range mutate {
		m(&deps)
	}
	h.srv = httptest.NewServer(httpapi.New(deps).Handler())
	t.Cleanup(h.srv.Close)
	return h
}

type client struct {
	h    *harness
	http *http.Client
	ip   string
}

// client cria um navegador de teste com cookies próprios e IP próprio.
func (h *harness) client() *client {
	jar, _ := cookiejar.New(nil)
	return &client{h: h, http: &http.Client{Jar: jar}, ip: fmt.Sprintf("198.51.100.%d", h.nextIP.Add(1))}
}

type rawBody string

type response struct {
	status int
	header http.Header
	body   []byte
}

func (r response) decode(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.body, v); err != nil {
		t.Fatalf("JSON inválido (%d): %s", r.status, r.body)
	}
}

func (r response) code() string {
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(r.body, &body)
	return body.Error.Code
}

// do envia a requisição; headers são pares nome/valor (valor vazio remove o header padrão).
func (c *client) do(method, path string, body any, headers ...string) response {
	c.h.t.Helper()
	var reader io.Reader
	switch b := body.(type) {
	case nil:
	case rawBody:
		reader = bytes.NewReader([]byte(b))
	default:
		raw, _ := json.Marshal(b)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.h.srv.URL+path, reader)
	if err != nil {
		c.h.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Origin", origin)
	req.Header.Set("X-Forwarded-For", c.ip)
	for i := 0; i+1 < len(headers); i += 2 {
		if headers[i+1] == "" {
			req.Header.Del(headers[i])
		} else {
			req.Header.Set(headers[i], headers[i+1])
		}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.h.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return response{status: resp.StatusCode, header: resp.Header, body: raw}
}

func (c *client) get(path string, headers ...string) response {
	return c.do(http.MethodGet, path, nil, headers...)
}
func (c *client) post(path string, body any, headers ...string) response {
	return c.do(http.MethodPost, path, body, headers...)
}

func expect(t *testing.T, r response, status int, code string) {
	t.Helper()
	if r.status != status || r.code() != code {
		t.Fatalf("status %d código %q, quer %d %q; corpo %s", r.status, r.code(), status, code, r.body)
	}
}

const password = "cavalo-bateria-grampo"

// signup cadastra, verifica o e-mail e faz login; devolve o cliente já autenticado.
func (h *harness) signup(username string) *client {
	h.t.Helper()
	c := h.client()
	email := username + "@empresa.com"
	r := c.post("/api/auth/register", map[string]string{"username": username, "email": email, "displayName": "Nome " + username, "password": password})
	if r.status != http.StatusAccepted {
		h.t.Fatalf("cadastro de %s: %d %s", username, r.status, r.body)
	}
	if r := c.post("/api/auth/verify-email", map[string]string{"token": h.mail.LastToken(email)}); r.status != http.StatusNoContent {
		h.t.Fatalf("verificação de %s: %d %s", username, r.status, r.body)
	}
	if r := c.post("/api/auth/login", map[string]string{"login": username, "password": password}); r.status != http.StatusOK {
		h.t.Fatalf("login de %s: %d %s", username, r.status, r.body)
	}
	return c
}

func (h *harness) makeAdmin(username string) {
	h.t.Helper()
	if _, err := h.pool.Exec(context.Background(), "UPDATE users SET is_admin = true WHERE username = $1", username); err != nil {
		h.t.Fatal(err)
	}
}

func (c *client) userID() string {
	c.h.t.Helper()
	var body struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	c.get("/api/me").decode(c.h.t, &body)
	return body.User.ID
}
