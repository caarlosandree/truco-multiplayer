package httpapi

import (
	"compress/gzip"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
)

const staticCSP = "default-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'"

const (
	cacheImmutable  = "public, max-age=31536000, immutable" // assets com hash do Vite
	cacheRevalidate = "no-cache"                            // index.html e demais arquivos
)

// compressibleTypes lista os tipos (sem parâmetros) que valem a pena comprimir.
// Áudio e fontes binárias ficam de fora de propósito.
var compressibleTypes = map[string]bool{
	"text/html":              true,
	"text/css":               true,
	"text/javascript":        true,
	"application/javascript": true,
	"application/json":       true,
	"image/svg+xml":          true,
}

var gzipPool = sync.Pool{New: func() any { return gzip.NewWriter(io.Discard) }}

// spaHandler serve o build do frontend e devolve index.html para rotas do cliente.
func spaHandler(dir string) http.Handler {
	fsys := os.DirFS(dir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", staticCSP)
		w.Header().Set("Vary", "Accept-Encoding")
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if info, err := fs.Stat(fsys, name); err != nil || info.IsDir() {
			name = "index.html"
		}
		serveStatic(w, r, fsys, name)
	})
}

// serveStatic responde um arquivo do build com cache por tipo de caminho e gzip quando aplicável.
func serveStatic(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) {
	cache := cacheRevalidate
	if strings.HasPrefix(name, "assets/") {
		cache = cacheImmutable
	}
	w.Header().Set("Cache-Control", cache)

	ctype := mime.TypeByExtension(path.Ext(name))
	if i := strings.IndexByte(ctype, ';'); i >= 0 {
		ctype = ctype[:i]
	}
	if r.Method == http.MethodGet && compressibleTypes[ctype] && acceptsGzip(r.Header.Get("Accept-Encoding")) {
		// Range aponta para bytes do arquivo sem compressão; como o corpo vai comprimido, serve-se o arquivo inteiro.
		r.Header.Del("Range")
		r.Header.Del("If-Range")
		gw := &gzipWriter{ResponseWriter: w}
		defer gw.close()
		http.ServeFileFS(gw, r, fsys, name)
		return
	}
	http.ServeFileFS(w, r, fsys, name)
}

// acceptsGzip interpreta Accept-Encoding: "gzip" explícito (inclusive q=0) tem precedência sobre "*".
func acceptsGzip(header string) bool {
	wildcard := false
	for _, part := range strings.Split(header, ",") {
		coding, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		coding = strings.ToLower(strings.TrimSpace(coding))
		if coding != "gzip" && coding != "*" {
			continue
		}
		accepted := qualityAccepted(params)
		if coding == "gzip" {
			return accepted
		}
		wildcard = accepted
	}
	return wildcard
}

func qualityAccepted(params string) bool {
	for _, p := range strings.Split(params, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(p), "=")
		if ok && strings.EqualFold(strings.TrimSpace(key), "q") {
			q, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			return err == nil && q > 0
		}
	}
	return true
}

// gzipWriter comprime o corpo só quando a resposta é 200; 304 e erros passam sem corpo comprimido.
type gzipWriter struct {
	http.ResponseWriter
	gz      *gzip.Writer
	started bool
}

func (g *gzipWriter) WriteHeader(code int) {
	if g.started {
		return
	}
	g.started = true
	if code == http.StatusOK {
		h := g.Header()
		h.Del("Content-Length")
		h.Set("Content-Encoding", "gzip")
		g.gz = gzipPool.Get().(*gzip.Writer)
		g.gz.Reset(g.ResponseWriter)
	}
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipWriter) Write(b []byte) (int, error) {
	if !g.started {
		g.WriteHeader(http.StatusOK)
	}
	if g.gz == nil {
		return g.ResponseWriter.Write(b)
	}
	return g.gz.Write(b)
}

func (g *gzipWriter) close() {
	if g.gz == nil {
		return
	}
	_ = g.gz.Close()
	gzipPool.Put(g.gz)
	g.gz = nil
}
