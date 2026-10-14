package httpapi_test

import (
	"net/http"
	"testing"
)

func TestAdminRoutesRequireAdmin(t *testing.T) {
	h := newHarness(t)
	user := h.signup("ana")
	anon := h.client()
	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/admin/users"},
		{http.MethodPost, "/api/admin/users/x/disable"},
		{http.MethodPost, "/api/admin/users/x/enable"},
		{http.MethodPost, "/api/admin/users/x/force-password-reset"},
		{http.MethodPut, "/api/admin/users/x/admin"},
		{http.MethodGet, "/api/admin/email-domains"},
		{http.MethodPost, "/api/admin/email-domains"},
		{http.MethodDelete, "/api/admin/email-domains/x.com"},
		{http.MethodPost, "/api/admin/matches/x/abort"},
		{http.MethodGet, "/api/admin/audit"},
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

	if r := boss.do(http.MethodPut, "/api/admin/users/"+anaID+"/admin", map[string]bool{"isAdmin": true}); r.status != http.StatusNoContent {
		t.Fatalf("set admin = %d %s", r.status, r.body)
	}
	expect(t, boss.do(http.MethodPut, "/api/admin/users/"+anaID+"/admin", map[string]any{}), http.StatusBadRequest, "invalid_request")
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
