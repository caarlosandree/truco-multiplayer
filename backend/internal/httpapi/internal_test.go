package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/admin"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/auth"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/config"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/match"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/pagination"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/replay"
	"github.com/caarlosandree/truco-multiplayer/backend/internal/stats"
)

func TestDecodeJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}
	cases := []struct {
		name, contentType, body string
		want                    error
	}{
		{"ok", "application/json", `{"name":"ana"}`, nil},
		{"charset", "application/json; charset=utf-8", `{"name":"ana"}`, nil},
		{"tipo errado", "text/plain", `{"name":"ana"}`, errUnsupportedMedia},
		{"campo desconhecido", "application/json", `{"name":"ana","seat":1}`, errInvalidJSON},
		{"malformado", "application/json", `{"name":`, errInvalidJSON},
		{"lixo depois", "application/json", `{"name":"ana"} {}`, errInvalidJSON},
		{"grande demais", "application/json", `{"name":"` + strings.Repeat("a", 17<<10) + `"}`, errBodyTooLarge},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodPost, "/api/x", strings.NewReader(c.body))
		r.Header.Set("Content-Type", c.contentType)
		var p payload
		if err := decodeJSON(httptest.NewRecorder(), r, &p); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, quer %v", c.name, err, c.want)
		}
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{auth.FieldErrors{"email": "inválido"}, 400, "validation_failed"},
		{auth.ErrEmailDomainNotAllowed, 403, "email_domain_not_allowed"},
		{auth.ErrUsernameTaken, 409, "username_taken"},
		{auth.ErrInvalidToken, 400, "invalid_token"},
		{auth.ErrInvalidCredentials, 401, "invalid_credentials"},
		{auth.ErrEmailNotVerified, 403, "email_not_verified"},
		{auth.ErrAccountDisabled, 403, "account_disabled"},
		{auth.ErrPasswordResetRequired, 403, "password_reset_required"},
		{auth.ErrUnauthenticated, 401, "unauthenticated"},
		{auth.ErrUserNotFound, 404, "not_found"},
		{stats.ErrUserNotFound, 404, "not_found"},
		{match.ErrNotFound, 404, "not_found"},
		{admin.ErrNotFound, 404, "not_found"},
		{pagination.ErrInvalidCursor, 400, "validation_failed"},
		{admin.ErrInvalidDomain, 400, "validation_failed"},
		{admin.ErrSelfAction, 409, "conflict"},
		{admin.ErrLastAdmin, 409, "conflict"},
		{admin.ErrLastDomain, 409, "conflict"},
		{match.ErrMatchClosed, 409, "conflict"},
		{&replay.CorruptLogError{Ply: 3, Err: errors.New("x")}, 409, "conflict"},
		{fmt.Errorf("embrulhado: %w", replay.ErrIncompatibleVersion), 409, "conflict"},
		{errMatchInProgress, 409, "match_in_progress"},
		{errors.New("qualquer outra coisa"), 500, "internal"},
	}
	for _, c := range cases {
		he := classify(c.err)
		if he.status != c.status || he.code != c.code {
			t.Errorf("%v: %d %s, quer %d %s", c.err, he.status, he.code, c.status, c.code)
		}
	}
}

func TestRecovererReturnsJSON500(t *testing.T) {
	s := &Server{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	h := s.recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/x", nil))
	var body struct {
		Error struct{ Code string } `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != 500 || body.Error.Code != "internal" {
		t.Fatalf("status %d corpo %s", rec.Code, rec.Body)
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.5:5555"
	r.Header.Set("X-Forwarded-For", "1.1.1.1, 203.0.113.9")
	direct := &Server{deps: Deps{Config: config.Config{TrustProxy: false}}}
	proxied := &Server{deps: Deps{Config: config.Config{TrustProxy: true}}}
	if got := direct.clientIP(r); got != "10.0.0.5" {
		t.Fatalf("sem proxy: %q", got)
	}
	if got := proxied.clientIP(r); got != "203.0.113.9" {
		t.Fatalf("com proxy: %q (usa o último hop)", got)
	}
	r.Header.Set("X-Forwarded-For", "lixo")
	if got := proxied.clientIP(r); got != "10.0.0.5" {
		t.Fatalf("XFF inválido deve cair no RemoteAddr: %q", got)
	}
}

func TestClientIPDropsZoneAndUnmaps(t *testing.T) {
	direct := &Server{deps: Deps{Config: config.Config{TrustProxy: false}}}
	proxied := &Server{deps: Deps{Config: config.Config{TrustProxy: true}}}
	for raw, want := range map[string]string{"fe80::1%eth0": "fe80::1", "::ffff:1.2.3.4": "1.2.3.4"} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "[" + raw + "]:5555"
		if got := direct.clientIP(r); got != want {
			t.Errorf("RemoteAddr %q: %q, quer %q", raw, got, want)
		}
		r.Header.Set("X-Forwarded-For", raw)
		if got := proxied.clientIP(r); got != want {
			t.Errorf("X-Forwarded-For %q: %q, quer %q", raw, got, want)
		}
	}
}
