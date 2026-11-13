package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// staticHandler serve o cliente já compilado (client/dist).
// Arquivos com hash em /assets/ ganham cache longo. Um caminho sem extensão que tenha
// página própria serve a página (/novidades → novidades.html, como /privacidade e
// /termos); o resto cai no index.html (SPA).
func staticHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := filepath.Clean("/" + r.URL.Path)
		path := filepath.Join(dir, clean)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			if strings.HasPrefix(clean, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			fs.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		if clean != "/" && filepath.Ext(clean) == "" {
			if info, err := os.Stat(path + ".html"); err == nil && !info.IsDir() {
				http.ServeFile(w, r, path+".html")
				return
			}
		}
		http.ServeFile(w, r, index)
	})
}
