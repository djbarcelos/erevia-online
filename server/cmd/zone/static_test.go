package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticHandlerPages(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"index.html":       "jogo",
		"novidades.html":   "novidades",
		"novidades.json":   `{"latest":"x"}`,
		"assets/app.js":    "js",
		"privacidade.html": "privacidade",
	}
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h := staticHandler(dir)
	cases := []struct{ path, want, cache string }{
		{"/", "jogo", "no-cache"},
		{"/novidades", "novidades", "no-cache"},
		{"/novidades.html", "novidades", "no-cache"},
		{"/novidades.json", `{"latest":"x"}`, "no-cache"},
		{"/privacidade", "privacidade", "no-cache"},
		{"/qualquer/rota", "jogo", "no-cache"},
		{"/assets", "jogo", "no-cache"},
		{"/assets/app.js", "js", "public, max-age=31536000, immutable"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
		body, _ := io.ReadAll(rec.Result().Body)
		if rec.Code != http.StatusOK || strings.TrimSpace(string(body)) != c.want {
			t.Errorf("%s: status %d, corpo %q; queria %q", c.path, rec.Code, body, c.want)
		}
		if got := rec.Header().Get("Cache-Control"); got != c.cache {
			t.Errorf("%s: Cache-Control %q; queria %q", c.path, got, c.cache)
		}
	}
}
