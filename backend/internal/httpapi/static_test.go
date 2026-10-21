package httpapi_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/httpapi"
)

const (
	bigSVG  = `<svg xmlns="http://www.w3.org/2000/svg">` + "<!-- carta -->" + "<path d=\"M0 0L10 10\"/>" + "</svg>"
	bigJS   = "console.log('truco');\n"
	spaHTML = "<html>spa</html>"
)

// staticDir monta um build falso com um arquivo hash, um SVG grande, um áudio e uma fonte.
func staticDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("index.html", spaHTML)
	write("assets/index-abc123.js", strings.Repeat(bigJS, 200))
	write("assets/carta.svg", strings.Repeat(bigSVG, 200))
	write("assets/som.ogg", strings.Repeat("ogg", 500))
	write("assets/fonte.woff2", strings.Repeat("woff", 500))
	return dir
}

func gunzip(t *testing.T, body []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("corpo não é gzip válido: %v", err)
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	return out
}

func TestStaticCacheHeadersByPathType(t *testing.T) {
	h := newHarness(t, func(d *httpapi.Deps) { d.Config.StaticDir = staticDir(t) })
	c := h.client()
	cases := map[string]string{
		"/assets/index-abc123.js": "public, max-age=31536000, immutable",
		"/assets/carta.svg":       "public, max-age=31536000, immutable",
		"/index.html":             "no-cache",
		"/":                       "no-cache",
		"/ranking/semanal":        "no-cache", // fallback da SPA
		"/assets/nao-existe.js":   "no-cache", // fallback também para assets inexistentes
	}
	for path, want := range cases {
		r := c.get(path, "Accept-Encoding", "identity")
		if r.status != http.StatusOK {
			t.Errorf("%s: status = %d", path, r.status)
		}
		if got := r.header.Get("Cache-Control"); got != want {
			t.Errorf("%s: Cache-Control = %q, quer %q", path, got, want)
		}
	}
}

func TestStaticGzipRespectsAcceptEncoding(t *testing.T) {
	h := newHarness(t, func(d *httpapi.Deps) { d.Config.StaticDir = staticDir(t) })
	c := h.client()
	original := strings.Repeat(bigSVG, 200)

	t.Run("gzip aceito comprime SVG e preserva o conteúdo", func(t *testing.T) {
		r := c.get("/assets/carta.svg", "Accept-Encoding", "gzip")
		if r.header.Get("Content-Encoding") != "gzip" {
			t.Fatalf("Content-Encoding = %q", r.header.Get("Content-Encoding"))
		}
		if r.header.Get("Vary") != "Accept-Encoding" {
			t.Errorf("Vary = %q", r.header.Get("Vary"))
		}
		if len(r.body) >= len(original) {
			t.Errorf("corpo comprimido (%d) não é menor que o original (%d)", len(r.body), len(original))
		}
		if got := string(gunzip(t, r.body)); got != original {
			t.Errorf("conteúdo descomprimido diferente do original")
		}
	})

	t.Run("sem gzip o corpo vem cru e ainda com Vary", func(t *testing.T) {
		r := c.get("/assets/carta.svg", "Accept-Encoding", "identity")
		if r.header.Get("Content-Encoding") != "" {
			t.Fatalf("Content-Encoding = %q", r.header.Get("Content-Encoding"))
		}
		if string(r.body) != original {
			t.Errorf("corpo cru diferente do original")
		}
		if r.header.Get("Vary") != "Accept-Encoding" {
			t.Errorf("Vary = %q", r.header.Get("Vary"))
		}
	})

	t.Run("gzip com q=0 não comprime", func(t *testing.T) {
		r := c.get("/assets/index-abc123.js", "Accept-Encoding", "gzip;q=0, identity")
		if r.header.Get("Content-Encoding") != "" {
			t.Fatalf("Content-Encoding = %q", r.header.Get("Content-Encoding"))
		}
	})

	t.Run("curinga * comprime", func(t *testing.T) {
		r := c.get("/index.html", "Accept-Encoding", "br, *;q=0.5")
		if r.header.Get("Content-Encoding") != "gzip" {
			t.Fatalf("Content-Encoding = %q", r.header.Get("Content-Encoding"))
		}
		if got := string(gunzip(t, r.body)); got != spaHTML {
			t.Errorf("index.html descomprimido = %q", got)
		}
	})

	t.Run("HEAD não comprime", func(t *testing.T) {
		r := c.do(http.MethodHead, "/assets/carta.svg", nil, "Accept-Encoding", "gzip")
		if r.header.Get("Content-Encoding") != "" {
			t.Fatalf("Content-Encoding = %q", r.header.Get("Content-Encoding"))
		}
	})
}

func TestStaticDoesNotCompressAudioOrFonts(t *testing.T) {
	mime.AddExtensionType(".ogg", "audio/ogg")
	mime.AddExtensionType(".woff2", "font/woff2")
	h := newHarness(t, func(d *httpapi.Deps) { d.Config.StaticDir = staticDir(t) })
	c := h.client()
	for _, path := range []string{"/assets/som.ogg", "/assets/fonte.woff2"} {
		r := c.get(path, "Accept-Encoding", "gzip, deflate, br")
		if r.status != http.StatusOK || r.header.Get("Content-Encoding") != "" {
			t.Errorf("%s: status=%d Content-Encoding=%q", path, r.status, r.header.Get("Content-Encoding"))
		}
	}
}

func TestStaticRangeStillWorksForUncompressedMedia(t *testing.T) {
	mime.AddExtensionType(".ogg", "audio/ogg")
	h := newHarness(t, func(d *httpapi.Deps) { d.Config.StaticDir = staticDir(t) })
	r := h.client().get("/assets/som.ogg", "Accept-Encoding", "gzip", "Range", "bytes=0-3")
	if r.status != http.StatusPartialContent || string(r.body) != "oggo" {
		t.Fatalf("Range em áudio: status=%d body=%q", r.status, r.body)
	}
}

func TestStaticNotModifiedIsNotCompressed(t *testing.T) {
	h := newHarness(t, func(d *httpapi.Deps) { d.Config.StaticDir = staticDir(t) })
	c := h.client()
	first := c.get("/assets/carta.svg", "Accept-Encoding", "identity")
	lastModified := first.header.Get("Last-Modified")
	if lastModified == "" {
		t.Fatal("sem Last-Modified para testar revalidação")
	}
	r := c.get("/assets/carta.svg", "Accept-Encoding", "gzip", "If-Modified-Since", lastModified)
	if r.status != http.StatusNotModified {
		t.Fatalf("status = %d, quer 304", r.status)
	}
	if r.header.Get("Content-Encoding") != "" || len(r.body) != 0 {
		t.Errorf("304 com Content-Encoding=%q e corpo de %d bytes", r.header.Get("Content-Encoding"), len(r.body))
	}
}

func TestStaticSPAFallbackWithGzip(t *testing.T) {
	h := newHarness(t, func(d *httpapi.Deps) { d.Config.StaticDir = staticDir(t) })
	r := h.client().get("/ranking/semanal", "Accept-Encoding", "gzip")
	if r.status != http.StatusOK || r.header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("fallback: status=%d Content-Encoding=%q", r.status, r.header.Get("Content-Encoding"))
	}
	if got := string(gunzip(t, r.body)); got != spaHTML {
		t.Errorf("fallback descomprimido = %q", got)
	}
	if !strings.Contains(r.header.Get("Content-Security-Policy"), "default-src 'self'") {
		t.Errorf("CSP ausente no fallback")
	}
}
