package app

import (
	"embed"
	"io/fs"
	"net/http"
)

// The tracked BUILD.txt lets Go tests run without a frontend toolchain.
// Docker copies the freshly built React bundle into web/dist before go build.
//
//go:embed web
var assets embed.FS

func frontend() http.Handler {
	files := frontendFiles()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		files.ServeHTTP(w, r)
	})
}

func frontendFiles() http.Handler {
	files, err := fs.Sub(assets, "web/dist")
	if err != nil {
		panic(err)
	}
	if _, err := fs.Stat(files, "index.html"); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			write(w, http.StatusServiceUnavailable, map[string]any{"error": "frontend bundle unavailable", "data_as_of": nil})
		})
	}
	return http.FileServer(http.FS(files))
}
