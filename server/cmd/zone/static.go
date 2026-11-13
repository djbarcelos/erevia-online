package main

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"erevia/server/internal/api"
)

// adminAssets é a pasta dos arquivos do portal no build do Vite (ver client/vite.config.ts).
const adminAssets = "/assets/admin/"

// isAdminPath: a página do portal (/admin, /admin/…, /admin.html) e os assets dela.
// Recebe o caminho já limpo e decodificado, então %2F, "//" e "/./" não escapam.
func isAdminPath(clean string) bool {
	return clean == "/admin" || clean == "/admin.html" || strings.HasPrefix(clean, "/admin/") ||
		strings.HasPrefix(clean+"/", adminAssets)
}

// staticHandler serve o cliente já compilado (client/dist).
//   - "/" é o jogo (index.html); arquivos que existem são servidos; o resto é 404 (antes
//     caía no index.html, e aí um 404 em /admin destoaria de qualquer outro caminho).
//   - Arquivos com hash em /assets/ ganham cache longo.
//   - Um caminho sem extensão que tenha página própria serve a página (/novidades →
//     novidades.html, como /privacidade e /termos).
//   - O portal (/admin e /assets/admin/) passa pelo adminOnly: só GM vê; para os outros é
//     o mesmo 404 de um caminho que não existe.
func staticHandler(dir string, adminOnly func(http.Handler) http.Handler) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")
	admin := adminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean("/" + r.URL.Path)
		if strings.HasPrefix(clean+"/", adminAssets) {
			serveFile(w, r, filepath.Join(dir, filepath.FromSlash(clean)))
			return
		}
		serveFile(w, r, filepath.Join(dir, "admin.html")) // as seções são #âncoras
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean("/" + r.URL.Path)
		if isAdminPath(clean) {
			admin.ServeHTTP(w, r)
			return
		}
		if clean == "/" {
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFile(w, r, index)
			return
		}
		if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(clean))); err == nil && !info.IsDir() {
			if strings.HasPrefix(clean, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			fs.ServeHTTP(w, r)
			return
		}
		// Página própria sem extensão: /novidades → novidades.html (e /privacidade, /termos).
		if path.Ext(clean) == "" {
			p := filepath.Join(dir, filepath.FromSlash(clean)+".html")
			if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
				w.Header().Set("Cache-Control", "no-cache")
				http.ServeFile(w, r, p)
				return
			}
		}
		api.NotFound(w, r)
	})
}

// serveFile serve um arquivo comum (nunca pasta nem listagem); o resto é 404.
func serveFile(w http.ResponseWriter, r *http.Request, p string) {
	if info, err := os.Stat(p); err != nil || !info.Mode().IsRegular() {
		api.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, p)
}
