package httpapi_test

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func registerBody(username, email string) map[string]string {
	return map[string]string{"username": username, "email": email, "displayName": "Nome", "password": password}
}

func TestFullAccountFlow(t *testing.T) {
	h := newHarness(t)
	c := h.signup("ana")
	var me struct {
		User struct {
			Username, Email, DisplayName string
			IsAdmin                      bool
		} `json:"user"`
	}
	r := c.get("/api/me")
	r.decode(t, &me)
	if r.status != 200 || me.User.Username != "ana" || me.User.Email != "ana@empresa.com" || me.User.IsAdmin {
		t.Fatalf("me = %d %s", r.status, r.body)
	}
	r = c.do(http.MethodPatch, "/api/me", map[string]string{"displayName": "Ana Clara"})
	r.decode(t, &me)
	if r.status != 200 || me.User.DisplayName != "Ana Clara" {
		t.Fatalf("patch = %d %s", r.status, r.body)
	}
	if r := c.post("/api/auth/logout", nil); r.status != http.StatusNoContent {
		t.Fatalf("logout = %d", r.status)
	}
	expect(t, c.get("/api/me"), http.StatusUnauthorized, "unauthenticated")
}

func TestLoginSetsSecureCookieAttributes(t *testing.T) {
	h := newHarness(t)
	c := h.client()
	c.post("/api/auth/register", registerBody("ana", "ana@empresa.com"))
	c.post("/api/auth/verify-email", map[string]string{"token": h.mail.LastToken("ana@empresa.com")})
	r := c.post("/api/auth/login", map[string]string{"login": "ana@empresa.com", "password": password})
	cookie := r.header.Get("Set-Cookie")
	for _, want := range []string{"truco_session=", "HttpOnly", "SameSite=Lax", "Path=/", "Max-Age=7776000"} {
		if !strings.Contains(cookie, want) {
			t.Fatalf("Set-Cookie sem %q: %s", want, cookie)
		}
	}
}

func TestInvalidCookieIsCleared(t *testing.T) {
	h := newHarness(t)
	r := h.client().get("/api/me", "Cookie", "truco_session=forjado")
	expect(t, r, http.StatusUnauthorized, "unauthenticated")
	if !strings.Contains(r.header.Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("cookie inválido não foi limpo: %v", r.header)
	}
}

func TestEnumerationResistantResponses(t *testing.T) {
	h := newHarness(t)
	h.signup("ana")
	fresh := h.client().post("/api/auth/register", registerBody("bia", "bia@empresa.com"))
	existing := h.client().post("/api/auth/register", registerBody("outra", "ana@empresa.com"))
	if fresh.status != existing.status || !bytes.Equal(fresh.body, existing.body) {
		t.Fatalf("cadastro revela e-mail existente: %d %s vs %d %s", fresh.status, fresh.body, existing.status, existing.body)
	}
	known := h.client().post("/api/auth/password/forgot", map[string]string{"email": "ana@empresa.com"})
	unknown := h.client().post("/api/auth/password/forgot", map[string]string{"email": "ninguem@empresa.com"})
	if known.status != http.StatusAccepted || known.status != unknown.status || !bytes.Equal(known.body, unknown.body) {
		t.Fatal("forgot revela e-mail existente")
	}
	resendKnown := h.client().post("/api/auth/resend-verification", map[string]string{"email": "bia@empresa.com"})
	resendUnknown := h.client().post("/api/auth/resend-verification", map[string]string{"email": "zzz@empresa.com"})
	if resendKnown.status != http.StatusAccepted || !bytes.Equal(resendKnown.body, resendUnknown.body) {
		t.Fatal("resend revela e-mail existente")
	}
	wrongUser := h.client().post("/api/auth/login", map[string]string{"login": "ninguem", "password": password})
	wrongPass := h.client().post("/api/auth/login", map[string]string{"login": "ana", "password": "senha-errada-123"})
	if !bytes.Equal(wrongUser.body, wrongPass.body) {
		t.Fatal("login diferencia usuário inexistente de senha errada")
	}
	expect(t, wrongPass, http.StatusUnauthorized, "invalid_credentials")
}

func TestRegisterErrors(t *testing.T) {
	h := newHarness(t)
	h.signup("ana")
	c := h.client()
	r := c.post("/api/auth/register", map[string]string{"username": "x", "email": "y", "displayName": "", "password": "z"})
	expect(t, r, http.StatusBadRequest, "validation_failed")
	var body struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	r.decode(t, &body)
	if len(body.Error.Fields) != 4 {
		t.Fatalf("fields = %v", body.Error.Fields)
	}
	expect(t, c.post("/api/auth/register", registerBody("bia", "bia@gmail.com")), http.StatusForbidden, "email_domain_not_allowed")
	expect(t, c.post("/api/auth/register", registerBody("ana", "outra@empresa.com")), http.StatusConflict, "username_taken")
	expect(t, c.post("/api/auth/register", rawBody(`{"username":"a","seat":2}`)), http.StatusBadRequest, "invalid_request")
	expect(t, c.post("/api/auth/verify-email", map[string]string{"token": "inexistente"}), http.StatusBadRequest, "invalid_token")
}

func TestLoginRequiresVerifiedEmail(t *testing.T) {
	h := newHarness(t)
	c := h.client()
	c.post("/api/auth/register", registerBody("ana", "ana@empresa.com"))
	expect(t, c.post("/api/auth/login", map[string]string{"login": "ana", "password": password}), http.StatusForbidden, "email_not_verified")
}

func TestLoginLockoutPerIdentifier(t *testing.T) {
	h := newHarness(t)
	h.signup("ana")
	for range 5 {
		expect(t, h.client().post("/api/auth/login", map[string]string{"login": "ana", "password": "senha-errada-123"}), http.StatusUnauthorized, "invalid_credentials")
	}
	r := h.client().post("/api/auth/login", map[string]string{"login": " ANA ", "password": password})
	expect(t, r, http.StatusTooManyRequests, "too_many_attempts")
	if r.header.Get("Retry-After") != "900" {
		t.Fatalf("Retry-After = %q", r.header.Get("Retry-After"))
	}
	h.clock.Advance(15 * time.Minute)
	if r := h.client().post("/api/auth/login", map[string]string{"login": "ana", "password": password}); r.status != http.StatusOK {
		t.Fatalf("após o bloqueio: %d %s", r.status, r.body)
	}
}

func TestPerIPRateLimits(t *testing.T) {
	h := newHarness(t)
	c := h.client()
	for i := range 10 {
		c.post("/api/auth/login", map[string]string{"login": "ninguem" + string(rune('a'+i)), "password": "x"})
	}
	expect(t, c.post("/api/auth/login", map[string]string{"login": "outro", "password": "x"}), http.StatusTooManyRequests, "too_many_attempts")

	reg := h.client()
	for i := range 5 {
		reg.post("/api/auth/register", registerBody("user"+string(rune('a'+i)), "u"+string(rune('a'+i))+"@empresa.com"))
	}
	expect(t, reg.post("/api/auth/register", registerBody("userz", "uz@empresa.com")), http.StatusTooManyRequests, "too_many_attempts")

	for range 3 {
		h.client().post("/api/auth/password/forgot", map[string]string{"email": "alvo@empresa.com"})
	}
	expect(t, h.client().post("/api/auth/password/forgot", map[string]string{"email": " ALVO@empresa.com"}), http.StatusTooManyRequests, "too_many_attempts")
}

func TestResetPasswordFlow(t *testing.T) {
	h := newHarness(t)
	ana := h.signup("ana")
	h.client().post("/api/auth/password/forgot", map[string]string{"email": "ana@empresa.com"})
	token := h.mail.LastToken("ana@empresa.com")
	other := h.client()
	expect(t, other.post("/api/auth/password/reset", map[string]string{"token": token, "password": "curta"}), http.StatusBadRequest, "validation_failed")
	if r := other.post("/api/auth/password/reset", map[string]string{"token": token, "password": "nova-senha-segura-1"}); r.status != http.StatusNoContent {
		t.Fatalf("reset = %d %s", r.status, r.body)
	}
	expect(t, ana.get("/api/me"), http.StatusUnauthorized, "unauthenticated")
	if r := other.post("/api/auth/login", map[string]string{"login": "ana", "password": "nova-senha-segura-1"}); r.status != http.StatusOK {
		t.Fatalf("login com nova senha = %d", r.status)
	}
}

func TestChangePasswordAndLogoutAll(t *testing.T) {
	h := newHarness(t)
	first := h.signup("ana")
	second := h.client()
	second.post("/api/auth/login", map[string]string{"login": "ana", "password": password})
	expect(t, first.post("/api/me/password", map[string]string{"currentPassword": "errada-errada-1", "newPassword": "nova-senha-segura-1"}), http.StatusUnauthorized, "invalid_credentials")
	r := first.post("/api/me/password", map[string]string{"currentPassword": password, "newPassword": "nova-senha-segura-1"})
	if r.status != http.StatusNoContent || !strings.Contains(r.header.Get("Set-Cookie"), "truco_session=") {
		t.Fatalf("troca = %d %v", r.status, r.header)
	}
	if r := first.get("/api/me"); r.status != 200 {
		t.Fatal("sessão rotacionada deve continuar valendo no mesmo navegador")
	}
	expect(t, second.get("/api/me"), http.StatusUnauthorized, "unauthenticated")

	second.post("/api/auth/login", map[string]string{"login": "ana", "password": "nova-senha-segura-1"})
	if r := first.post("/api/auth/logout-all", nil); r.status != http.StatusNoContent {
		t.Fatalf("logout-all = %d", r.status)
	}
	expect(t, first.get("/api/me"), http.StatusUnauthorized, "unauthenticated")
	expect(t, second.get("/api/me"), http.StatusUnauthorized, "unauthenticated")
	expect(t, h.client().post("/api/auth/logout-all", nil), http.StatusUnauthorized, "unauthenticated")
}

func TestPasswordResetRequiredBlocksLogin(t *testing.T) {
	h := newHarness(t)
	h.signup("ana")
	h.makeAdmin("ana") // nem administrador escapa da redefinição obrigatória
	_, _ = h.pool.Exec(t.Context(), "UPDATE users SET must_reset_password = true WHERE username = 'ana'")
	expect(t, h.client().post("/api/auth/login", map[string]string{"login": "ana", "password": password}), http.StatusForbidden, "password_reset_required")
}

// Username é público: ocupado responde 409 exista ou não o e-mail informado.
func TestRegisterTakenUsernameDoesNotRevealEmail(t *testing.T) {
	h := newHarness(t)
	h.signup("ana")
	h.signup("bia")
	existingEmail := h.client().post("/api/auth/register", registerBody("ana", "bia@empresa.com"))
	freshEmail := h.client().post("/api/auth/register", registerBody("ana", "nova@empresa.com"))
	expect(t, existingEmail, http.StatusConflict, "username_taken")
	if !bytes.Equal(existingEmail.body, freshEmail.body) || existingEmail.status != freshEmail.status {
		t.Fatalf("respostas diferentes: %d %s vs %d %s", existingEmail.status, existingEmail.body, freshEmail.status, freshEmail.body)
	}
}

func TestChangePasswordLockout(t *testing.T) {
	h := newHarness(t)
	ana := h.signup("ana")
	wrong := map[string]string{"currentPassword": "errada-errada-1", "newPassword": "nova-senha-segura-1"}
	right := map[string]string{"currentPassword": password, "newPassword": "nova-senha-segura-1"}
	for range 5 {
		expect(t, ana.post("/api/me/password", wrong), http.StatusUnauthorized, "invalid_credentials")
	}
	r := ana.post("/api/me/password", right)
	expect(t, r, http.StatusTooManyRequests, "too_many_attempts")
	if r.header.Get("Retry-After") != "900" {
		t.Fatalf("Retry-After = %q", r.header.Get("Retry-After"))
	}
	// O bloqueio da troca de senha não afeta o login (chaves independentes).
	if r := h.client().post("/api/auth/login", map[string]string{"login": "ana", "password": password}); r.status != http.StatusOK {
		t.Fatalf("login durante bloqueio da troca: %d %s", r.status, r.body)
	}
	h.clock.Advance(15 * time.Minute)
	if r := ana.post("/api/me/password", right); r.status != http.StatusNoContent {
		t.Fatalf("após o bloqueio: %d %s", r.status, r.body)
	}
}

func TestLoginLockoutHoldsUnderConcurrentBurst(t *testing.T) {
	h := newHarness(t)
	h.signup("ana")
	const burst = 10
	var (
		wg       sync.WaitGroup
		start    = make(chan struct{})
		mu       sync.Mutex
		statuses = map[int]int{}
	)
	for i := range burst {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := strings.NewReader(`{"login":"ana","password":"senha-errada-123"}`)
			req, _ := http.NewRequest(http.MethodPost, h.srv.URL+"/api/auth/login", body)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", origin)
			req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", i+1))
			<-start
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Error(err)
				return
			}
			resp.Body.Close()
			mu.Lock()
			statuses[resp.StatusCode]++
			mu.Unlock()
		}()
	}
	close(start)
	wg.Wait()
	if statuses[http.StatusUnauthorized] > 5 || statuses[http.StatusUnauthorized]+statuses[http.StatusTooManyRequests] != burst {
		t.Fatalf("rajada de %d logins errados: %v (no máximo 5 verificações de senha)", burst, statuses)
	}
	expect(t, h.client().post("/api/auth/login", map[string]string{"login": "ana", "password": password}), http.StatusTooManyRequests, "too_many_attempts")
}
