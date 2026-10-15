package httpapi_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/httpapi"
)

func TestHealthz(t *testing.T) {
	h := newHarness(t)
	r := h.client().get("/healthz")
	if r.status != http.StatusOK || !strings.Contains(string(r.body), "ok") {
		t.Fatalf("healthz = %d %s", r.status, r.body)
	}
}

func TestUnknownAPIRouteIsJSONWithSecurityHeaders(t *testing.T) {
	h := newHarness(t)
	r := h.client().get("/api/nao-existe")
	expect(t, r, http.StatusNotFound, "not_found")
	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff", "Referrer-Policy": "no-referrer", "X-Frame-Options": "DENY", "Cache-Control": "no-store",
	} {
		if got := r.header.Get(header); got != want {
			t.Errorf("%s = %q, quer %q", header, got, want)
		}
	}
	if r.header.Get("X-Request-Id") == "" || !strings.HasPrefix(r.header.Get("Content-Type"), "application/json") {
		t.Fatalf("headers = %v", r.header)
	}
}

func TestMutatingRequestsRequireOrigin(t *testing.T) {
	h := newHarness(t)
	c := h.client()
	expect(t, c.post("/api/qualquer", nil, "Origin", ""), http.StatusForbidden, "forbidden_origin")
	expect(t, c.post("/api/qualquer", nil, "Origin", "https://evil.test"), http.StatusForbidden, "forbidden_origin")
	expect(t, c.post("/api/qualquer", nil, "Origin", "", "Sec-Fetch-Site", "same-origin"), http.StatusNotFound, "not_found")
	expect(t, c.get("/api/qualquer", "Origin", ""), http.StatusNotFound, "not_found") // GET não exige Origin
}

func TestStaticSPAFallback(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>spa</html>"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "assets"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log(1)"), 0o644)
	h := newHarness(t, func(d *httpapi.Deps) { d.Config.StaticDir = dir })
	c := h.client()
	if r := c.get("/assets/app.js"); r.status != 200 || string(r.body) != "console.log(1)" {
		t.Fatalf("arquivo: %d %s", r.status, r.body)
	}
	r := c.get("/ranking/semanal")
	if r.status != 200 || !strings.Contains(string(r.body), "spa") || !strings.Contains(r.header.Get("Content-Security-Policy"), "default-src 'self'") {
		t.Fatalf("fallback: %d %s %v", r.status, r.body, r.header)
	}
	expect(t, c.get("/api/nao-existe"), http.StatusNotFound, "not_found")
}

func TestWebSocketRouteIsMountedWhenConfigured(t *testing.T) {
	h := newHarness(t, func(d *httpapi.Deps) {
		d.WS = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })
	})
	if r := h.client().get("/ws"); r.status != http.StatusTeapot {
		t.Fatalf("GET /ws = %d", r.status)
	}
}
