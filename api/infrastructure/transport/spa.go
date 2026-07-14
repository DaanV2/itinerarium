package transport

import (
	"io/fs"
	"net/http"
	"strings"
)

// SPAHandler serves the embedded frontend build. Real files are served as-is
// (SvelteKit's content-hashed assets get far-future caching); every other
// path falls back to the SPA shell so client-side routes survive deep links
// and reloads. Unmatched /api paths stay 404s — the shell must never mask a
// missing endpoint.
func SPAHandler(assets fs.FS) http.Handler {
	fileServer := http.FileServerFS(assets)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "api" || strings.HasPrefix(path, "api/") {
			http.NotFound(w, r)

			return
		}

		if path != "" {
			if info, err := fs.Stat(assets, path); err == nil && !info.IsDir() {
				if strings.HasPrefix(path, "_app/immutable/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}

				fileServer.ServeHTTP(w, r)

				return
			}
		}

		http.ServeFileFS(w, r, assets, "index.html")
	})
}
