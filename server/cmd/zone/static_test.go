package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

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
	h := staticHandler(dir)
	get := func(path string) (int, string) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
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
	if code, body := get("/qualquer/rota"); code != http.StatusOK || body != "<html>jogo</html>" {
		t.Fatalf("SPA = %d %q", code, body)
	}
}
