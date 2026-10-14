package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthenticateSlidingWindowAndAbsoluteCap(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	userID := f.newUser(t, "ana", "senha-da-ana-123")
	token, err := f.svc.createSession(ctx, f.svc.q, userID, SessionMeta{UserAgent: "teste", IP: "203.0.113.7"})
	if err != nil {
		t.Fatal(err)
	}
	for day := 29; day <= 87; day += 29 { // usa a cada 29 dias: a janela de 30 dias desliza
		f.clock.Advance(29 * 24 * time.Hour)
		p, err := f.svc.Authenticate(ctx, token)
		if err != nil || p.UserID != userID || p.Username != "ana" {
			t.Fatalf("dia %d: p=%+v err=%v", day, p, err)
		}
	}
	f.clock.Advance(4 * 24 * time.Hour) // dia 91: passou do teto absoluto de 90 dias
	if _, err := f.svc.Authenticate(ctx, token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("após 90 dias: err = %v", err)
	}
}

func TestAuthenticateExpiresAfterIdleWindow(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	token, _ := f.svc.createSession(ctx, f.svc.q, f.newUser(t, "ana", "senha-da-ana-123"), SessionMeta{})
	f.clock.Advance(30*24*time.Hour + time.Second)
	if _, err := f.svc.Authenticate(ctx, token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err = %v", err)
	}
}

func TestAuthenticateTouchesAtMostDaily(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	token, _ := f.svc.createSession(ctx, f.svc.q, f.newUser(t, "ana", "senha-da-ana-123"), SessionMeta{})
	lastSeen := func() time.Time {
		var ts time.Time
		if err := f.pool.QueryRow(ctx, "SELECT last_seen_at FROM sessions").Scan(&ts); err != nil {
			t.Fatal(err)
		}
		return ts.UTC()
	}
	f.clock.Advance(23 * time.Hour)
	_, _ = f.svc.Authenticate(ctx, token)
	if !lastSeen().Equal(t0) {
		t.Fatalf("tocou antes de 24h: %v", lastSeen())
	}
	f.clock.Advance(2 * time.Hour)
	_, _ = f.svc.Authenticate(ctx, token)
	if !lastSeen().Equal(t0.Add(25 * time.Hour)) {
		t.Fatalf("não tocou após 24h: %v", lastSeen())
	}
}

func TestAuthenticateRejectsDisabledUserEvenWithLiveSession(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	userID := f.newUser(t, "ana", "senha-da-ana-123")
	token, _ := f.svc.createSession(ctx, f.svc.q, userID, SessionMeta{})
	if _, err := f.pool.Exec(ctx, "UPDATE users SET disabled_at = now() WHERE id = $1", userID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(ctx, token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err = %v", err)
	}
}

func TestLogoutAndLogoutAll(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	userID := f.newUser(t, "ana", "senha-da-ana-123")
	a, _ := f.svc.createSession(ctx, f.svc.q, userID, SessionMeta{})
	b, _ := f.svc.createSession(ctx, f.svc.q, userID, SessionMeta{})
	if err := f.svc.Logout(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(ctx, a); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("logout não revogou a sessão")
	}
	if _, err := f.svc.Authenticate(ctx, b); err != nil {
		t.Fatal("logout revogou a sessão errada")
	}
	if err := f.svc.LogoutAll(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(ctx, b); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("logout-all não revogou")
	}
	if _, err := f.svc.Authenticate(ctx, ""); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("token vazio deve ser não autenticado")
	}
}

func TestCleanupExpired(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	userID := f.newUser(t, "ana", "senha-da-ana-123")
	_, _ = f.svc.createSession(ctx, f.svc.q, userID, SessionMeta{})
	f.clock.Advance(31 * 24 * time.Hour)
	_, _ = f.svc.createSession(ctx, f.svc.q, userID, SessionMeta{})
	sessions, _, err := f.svc.CleanupExpired(ctx)
	if err != nil || sessions != 1 {
		t.Fatalf("removidas %d, err %v", sessions, err)
	}
}

func TestCookieConfig(t *testing.T) {
	rec := httptest.NewRecorder()
	CookieConfig{Secure: true}.Set(rec, "tok")
	c := rec.Result().Cookies()[0]
	if c.Name != "__Host-truco_session" || c.Value != "tok" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge != 7776000 {
		t.Fatalf("cookie = %+v", c)
	}
	if (CookieConfig{Secure: false}).Name() != "truco_session" {
		t.Fatal("nome sem Secure")
	}
	rec = httptest.NewRecorder()
	CookieConfig{Secure: true}.Clear(rec)
	if c := rec.Result().Cookies()[0]; c.MaxAge >= 0 || c.Value != "" {
		t.Fatalf("Clear = %+v", c)
	}
}

func TestSessionFromRequest(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	token, _ := f.svc.createSession(ctx, f.svc.q, f.newUser(t, "ana", "senha-da-ana-123"), SessionMeta{})
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.AddCookie(&http.Cookie{Name: "__Host-truco_session", Value: token})
	p, err := f.svc.SessionFromRequest(r)
	if err != nil || p.Username != "ana" {
		t.Fatalf("p=%+v err=%v", p, err)
	}
	if _, err := f.svc.SessionFromRequest(httptest.NewRequest(http.MethodGet, "/ws", nil)); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("sem cookie: err = %v", err)
	}
}

func TestCheckOrigin(t *testing.T) {
	const allowed = "https://truco.test"
	cases := []struct {
		origin, fetchSite string
		want              bool
	}{
		{"https://truco.test", "", true},
		{"https://evil.test", "same-origin", false},
		{"null", "", false},
		{"", "same-origin", true},
		{"", "cross-site", false},
		{"", "", false},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
		if c.origin != "" {
			r.Header.Set("Origin", c.origin)
		}
		if c.fetchSite != "" {
			r.Header.Set("Sec-Fetch-Site", c.fetchSite)
		}
		if got := CheckOrigin(r, allowed); got != c.want {
			t.Errorf("origin=%q site=%q: %v, quer %v", c.origin, c.fetchSite, got, c.want)
		}
	}
}

func TestBootstrapDomains(t *testing.T) {
	f := newFixture(t) // já populou "empresa.com"
	ctx := context.Background()
	if err := f.svc.BootstrapDomains(ctx, []string{"outra.com"}); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = f.pool.QueryRow(ctx, "SELECT count(*) FROM allowed_email_domains WHERE domain = 'outra.com'").Scan(&n)
	if n != 0 {
		t.Fatal("bootstrap só pode popular tabela vazia")
	}
	_, _ = f.pool.Exec(ctx, "DELETE FROM allowed_email_domains")
	if err := f.svc.BootstrapDomains(ctx, nil); err == nil {
		t.Fatal("tabela vazia sem ALLOWED_EMAIL_DOMAINS deve falhar")
	}
}

func TestCreateSessionToleratesInvalidIP(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	userID := f.newUser(t, "ana", "senha-da-ana-123")
	for _, ip := range []string{"", "lixo", "::ffff:203.0.113.7"} {
		if _, err := f.svc.createSession(ctx, f.svc.q, userID, SessionMeta{IP: ip}); err != nil {
			t.Fatalf("ip %q: %v", ip, err)
		}
	}
}
