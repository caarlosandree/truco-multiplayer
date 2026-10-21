package httpapi_test

import (
	"context"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/httpapi"
)

var placeholder = regexp.MustCompile(`\{[^}]+\}`)

func newPatterns() []string {
	return httpapi.New(httpapi.Deps{WS: http.NotFoundHandler()}).Patterns()
}

func adminPatterns() []string {
	var out []string
	for _, p := range newPatterns() {
		if _, path, _ := strings.Cut(p, " "); strings.HasPrefix(path, "/api/admin/") {
			out = append(out, p)
		}
	}
	return out
}

func TestAdminRoutesRequireAdmin(t *testing.T) {
	h := newHarness(t)
	user := h.signup("ana")
	anon := h.client()
	// A lista vem do roteador: rota /api/admin/* nova entra no teste sem edição manual.
	var routes []struct{ method, path string }
	for _, p := range adminPatterns() {
		method, path, _ := strings.Cut(p, " ")
		routes = append(routes, struct{ method, path string }{method, placeholder.ReplaceAllString(path, "x")})
	}
	if len(routes) != 10 { // cardinalidade: mudou o conjunto de rotas admin, revise de propósito
		t.Fatalf("rotas admin = %d (%v), quer 10", len(routes), routes)
	}
	for _, rt := range routes {
		expect(t, user.do(rt.method, rt.path, nil), http.StatusForbidden, "forbidden")
		expect(t, anon.do(rt.method, rt.path, nil), http.StatusUnauthorized, "unauthenticated")
	}
}

func TestAdminUserManagement(t *testing.T) {
	h := newHarness(t)
	boss := h.signup("chefe")
	h.makeAdmin("chefe")
	ana := h.signup("ana")
	anaID := ana.userID()

	var page struct {
		Items []struct {
			Username string `json:"username"`
			IsAdmin  bool   `json:"isAdmin"`
		} `json:"items"`
	}
	r := boss.get("/api/admin/users?q=an")
	r.decode(t, &page)
	if r.status != 200 || len(page.Items) != 1 || page.Items[0].Username != "ana" {
		t.Fatalf("busca = %d %s", r.status, r.body)
	}

	if r := boss.post("/api/admin/users/"+anaID+"/disable", nil); r.status != http.StatusNoContent {
		t.Fatalf("disable = %d %s", r.status, r.body)
	}
	expect(t, ana.get("/api/me"), http.StatusUnauthorized, "unauthenticated")
	expect(t, h.client().post("/api/auth/login", map[string]string{"login": "ana", "password": password}), http.StatusForbidden, "account_disabled")
	if r := boss.post("/api/admin/users/"+anaID+"/enable", nil); r.status != http.StatusNoContent {
		t.Fatalf("enable = %d", r.status)
	}

	if r := boss.post("/api/admin/users/"+anaID+"/force-password-reset", nil); r.status != http.StatusNoContent {
		t.Fatalf("force reset = %d", r.status)
	}
	expect(t, h.client().post("/api/auth/login", map[string]string{"login": "ana", "password": password}), http.StatusForbidden, "password_reset_required")

	expect(t, boss.post("/api/admin/users/"+boss.userID()+"/disable", nil), http.StatusConflict, "conflict")
	expect(t, boss.post("/api/admin/users/nao-e-uuid/disable", nil), http.StatusNotFound, "not_found")
}

func TestAdminDomains(t *testing.T) {
	h := newHarness(t)
	boss := h.signup("chefe")
	h.makeAdmin("chefe")
	expect(t, boss.post("/api/admin/email-domains", map[string]string{"domain": "sem-ponto"}), http.StatusBadRequest, "validation_failed")
	if r := boss.post("/api/admin/email-domains", map[string]string{"domain": "filial.com.br"}); r.status != http.StatusNoContent {
		t.Fatalf("add = %d %s", r.status, r.body)
	}
	var domains struct {
		Items []struct {
			Domain string `json:"domain"`
		} `json:"items"`
	}
	boss.get("/api/admin/email-domains").decode(t, &domains)
	if len(domains.Items) != 2 {
		t.Fatalf("domínios = %+v", domains)
	}
	if r := h.client().post("/api/auth/register", registerBody("bia", "bia@filial.com.br")); r.status != http.StatusAccepted {
		t.Fatalf("domínio novo deve aceitar cadastro: %d %s", r.status, r.body)
	}
	if r := boss.do(http.MethodDelete, "/api/admin/email-domains/empresa.com", nil); r.status != http.StatusNoContent {
		t.Fatalf("remove = %d", r.status)
	}
	expect(t, boss.do(http.MethodDelete, "/api/admin/email-domains/filial.com.br", nil), http.StatusConflict, "conflict")
}

func TestAdminAbortMatchAndAudit(t *testing.T) {
	h := newHarness(t)
	tb := h.table()
	h.makeAdmin("ana")
	boss := tb.clients[0]
	id, _ := h.playMatch(tb, 7, 10)
	if r := boss.post("/api/admin/matches/"+id+"/abort", map[string]string{"reason": "teste"}); r.status != http.StatusNoContent {
		t.Fatalf("abort = %d %s", r.status, r.body)
	}
	expect(t, boss.post("/api/admin/matches/"+id+"/abort", map[string]string{"reason": "de novo"}), http.StatusConflict, "conflict")
	if r := boss.get("/api/matches/" + id + "/replay"); r.status != http.StatusOK {
		t.Fatalf("replay de partida abortada = %d", r.status)
	}
	var audit struct {
		Items []struct {
			Action string `json:"action"`
		} `json:"items"`
	}
	boss.get("/api/admin/audit").decode(t, &audit)
	if len(audit.Items) != 1 || audit.Items[0].Action != "match.abort" {
		t.Fatalf("auditoria = %+v", audit)
	}
}

func TestNoRouteChangesAdminFlag(t *testing.T) {
	h := newHarness(t)
	// Camada extra: nenhum padrão sugere mudança de privilégio. O trecho após /api/admin/ também
	// não pode terminar em "admin" (com ou sem barra).
	forbidden := regexp.MustCompile(`(?i)role|grant|privileg|elevat|permission|is_?admin|promote|demote`)
	adminSuffix := regexp.MustCompile(`(?i)admin(/|$)`)
	for _, p := range newPatterns() {
		if forbidden.MatchString(p) {
			t.Errorf("rota %q pode alterar privilégios", p)
		}
		if _, path, _ := strings.Cut(p, " "); strings.HasPrefix(path, "/api/admin/") && adminSuffix.MatchString(strings.TrimPrefix(path, "/api/admin/")) {
			t.Errorf("rota %q pode alterar is_admin", p)
		}
	}
	// Lista fechada (golden): qualquer rota nova ou removida quebra este teste de propósito.
	// Acrescentar uma rota exige revisão consciente da anti-meta (ninguém vira admin sem o dono).
	want := []string{
		"/",
		"DELETE /api/admin/email-domains/{domain}",
		"GET /api/admin/audit",
		"GET /api/admin/email-domains",
		"GET /api/admin/matches",
		"GET /api/admin/users",
		"GET /api/matches",
		"GET /api/matches/{id}",
		"GET /api/matches/{id}/replay",
		"GET /api/me",
		"GET /api/ranking",
		"GET /api/users",
		"GET /api/users/{username}",
		"GET /healthz",
		"GET /ws",
		"PATCH /api/me",
		"POST /api/admin/email-domains",
		"POST /api/admin/matches/{id}/abort",
		"POST /api/admin/users/{id}/disable",
		"POST /api/admin/users/{id}/enable",
		"POST /api/admin/users/{id}/force-password-reset",
		"POST /api/auth/login",
		"POST /api/auth/logout",
		"POST /api/auth/logout-all",
		"POST /api/auth/password/forgot",
		"POST /api/auth/password/reset",
		"POST /api/auth/register",
		"POST /api/auth/resend-verification",
		"POST /api/auth/verify-email",
		"POST /api/me/password",
	}
	got := slices.Clone(newPatterns())
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("rotas mudaram; revise a anti-meta antes de atualizar a lista.\ngot:  %q\nwant: %q", got, want)
	}
	boss := h.signup("chefe")
	h.makeAdmin("chefe")
	ana := h.signup("ana")
	anaID := ana.userID()
	for _, m := range []string{http.MethodPut, http.MethodPatch, http.MethodPost} {
		r := boss.do(m, "/api/admin/users/"+anaID+"/admin", map[string]bool{"isAdmin": true})
		if r.status != http.StatusNotFound && r.status != http.StatusMethodNotAllowed {
			t.Errorf("%s ...//admin = %d, quer 404/405", m, r.status)
		}
	}
	var isAdmin bool
	if err := h.pool.QueryRow(context.Background(), "SELECT is_admin FROM users WHERE id = $1", anaID).Scan(&isAdmin); err != nil {
		t.Fatal(err)
	}
	if isAdmin {
		t.Fatal("alvo virou admin por rota HTTP")
	}
}

func TestAdminCannotActOnAdmin(t *testing.T) {
	h := newHarness(t)
	boss := h.signup("chefe")
	h.makeAdmin("chefe")
	h.signup("outro")
	h.makeAdmin("outro")
	var otherID string
	if err := h.pool.QueryRow(context.Background(), "SELECT id::text FROM users WHERE username = 'outro'").Scan(&otherID); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"disable", "enable", "force-password-reset"} {
		expect(t, boss.post("/api/admin/users/"+otherID+"/"+action, nil), http.StatusForbidden, "target_is_admin")
	}
	var n int
	if err := h.pool.QueryRow(context.Background(), "SELECT count(*) FROM audit_log").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("auditoria = %d, quer 0", n)
	}
}

func TestAdminInProgressMatches(t *testing.T) {
	h := newHarness(t)
	tb := h.table()
	h.makeAdmin("ana")
	boss := tb.clients[0]
	id, _ := h.playMatch(tb, 7, 3)
	var page struct {
		Items []struct {
			ID      string `json:"id"`
			Players []struct {
				Username string `json:"username"`
			} `json:"players"`
		} `json:"items"`
	}
	r := boss.get("/api/admin/matches")
	r.decode(t, &page)
	if r.status != 200 || len(page.Items) != 1 || page.Items[0].ID != id || len(page.Items[0].Players) != 4 {
		t.Fatalf("em andamento = %d %s", r.status, r.body)
	}
	if r := boss.get("/api/admin/matches?status=in_progress"); r.status != 200 {
		t.Fatalf("status explícito = %d", r.status)
	}
	expect(t, boss.get("/api/admin/matches?status=finished"), http.StatusBadRequest, "invalid_request")
	if r := boss.get("/api/admin/matches?cursor=%25%25%25"); r.status != http.StatusBadRequest {
		t.Fatalf("cursor inválido = %d %s", r.status, r.body)
	}
}
