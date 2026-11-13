package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"erevia/server/internal/api"
)

// fakeAdminOnly faz o papel do api.AdminOnly: só passa quem manda X-Test-GM.
func fakeAdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test-GM") != "1" {
			api.NotFound(w, r)
			return
		}
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func TestStaticHidesAdminFromNonGM(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"index.html":                  "jogo",
		"admin.html":                  "portal",
		"privacidade.html":            "privacidade",
		"assets/main-abc.js":          "js do jogo",
		"assets/cinzel-latin.woff2":   "fonte",
		"assets/admin/admin-abc.js":   "js do portal",
		"assets/admin/admin-abc.css":  "css do portal",
		"assets/admin/sub/nested.txt": "aninhado",
	} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mux := http.NewServeMux()
	mux.Handle("/", staticHandler(dir, fakeAdminOnly))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	get := func(path string, gm bool) (int, string, http.Header) {
		t.Helper()
		req, _ := http.NewRequest("GET", srv.URL+path, nil)
		if gm {
			req.Header.Set("X-Test-GM", "1")
		}
		c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b), resp.Header
	}

	// O jogo e os arquivos públicos continuam iguais.
	for path, want := range map[string]string{
		"/": "jogo", "/privacidade.html": "privacidade", "/assets/main-abc.js": "js do jogo", "/assets/cinzel-latin.woff2": "fonte",
	} {
		if code, body, _ := get(path, false); code != http.StatusOK || body != want {
			t.Errorf("%s = %d %q, want %q", path, code, body, want)
		}
	}

	// Caminho que não existe: 404 (não cai mais no index.html), igual ao do portal escondido.
	code, notFound, hdr := get("/nao-existe", false)
	if code != http.StatusNotFound || hdr.Get("Cache-Control") != "no-store" {
		t.Fatalf("/nao-existe = %d %q %v", code, notFound, hdr)
	}

	admin := map[string]string{
		"/admin": "portal", "/admin/": "portal", "/admin/jogadores": "portal", "/admin.html": "portal",
		"/assets/admin/admin-abc.js": "js do portal", "/assets/admin/admin-abc.css": "css do portal",
		"/assets/admin/sub/nested.txt": "aninhado",
	}
	for path := range admin {
		code, body, hdr := get(path, false)
		if code != http.StatusNotFound || body != notFound || hdr.Get("X-Frame-Options") != "" {
			t.Errorf("non-GM %s = %d %q", path, code, body)
		}
	}
	// Caminhos codificados ou tortos não escapam da checagem.
	for _, path := range []string{"/assets%2Fadmin%2Fadmin-abc.js", "/assets/admin%2Fadmin-abc.js", "/%61dmin.html", "/assets/./admin/admin-abc.js", "/x/../admin.html"} {
		if code, body, _ := get(path, false); code == http.StatusOK || body == "js do portal" || body == "portal" {
			t.Errorf("non-GM %s = %d %q (leaked)", path, code, body)
		}
	}
	// Pasta não lista arquivos, nem para GM.
	if code, _, _ := get("/assets/admin/", true); code != http.StatusNotFound {
		t.Errorf("GM /assets/admin/ = %d, want 404", code)
	}
	if code, _, _ := get("/assets/", false); code != http.StatusNotFound {
		t.Errorf("/assets/ = %d, want 404", code)
	}

	for path, want := range admin {
		code, body, hdr := get(path, true)
		if code != http.StatusOK || body != want || hdr.Get("X-Frame-Options") != "DENY" {
			t.Errorf("GM %s = %d %q", path, code, body)
		}
	}
}

// Páginas sem extensão (/novidades, /privacidade) servem a página própria; um caminho que
// não existe é 404 (não cai mais no index.html).
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
	h := staticHandler(dir, fakeAdminOnly)
	cases := []struct{ path, want, cache string }{
		{"/", "jogo", "no-cache"},
		{"/novidades", "novidades", "no-cache"},
		{"/novidades.html", "novidades", "no-cache"},
		{"/novidades.json", `{"latest":"x"}`, "no-cache"},
		{"/privacidade", "privacidade", "no-cache"},
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
	for _, p := range []string{"/qualquer/rota", "/assets", "/nada"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d; queria 404", p, rec.Code)
		}
	}
}

// Os sourcemaps não podem sair para o público, nem se estiverem na pasta servida.
func TestStaticHidesSourcemaps(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"index.html":              "<html>jogo</html>",
		"assets/index-abc.js":     "console.log(1)",
		"assets/index-abc.js.map": `{"version":3}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h := staticHandler(dir, fakeAdminOnly)
	get := func(p string) (int, string) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		return rec.Code, rec.Body.String()
	}
	if code, body := get("/assets/index-abc.js"); code != http.StatusOK || body != "console.log(1)" {
		t.Fatalf("js = %d %q", code, body)
	}
	for _, p := range []string{"/assets/index-abc.js.map", "/assets/INDEX-abc.js.MAP", "/assets/../assets/index-abc.js.map", "/assets/nada.js.map"} {
		if code, _ := get(p); code != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", p, code)
		}
	}
}
