package transport

import (
	"net/http"
	"time"

	"github.com/DaanV2/itinerarium/api/infrastructure/logging"
	"github.com/charmbracelet/log"
)

// Logging attaches a request-scoped logger (method and path) to the request
// context, so downstream handlers and services can retrieve it via
// logging.From, and logs one summary line per request: method, path,
// duration.
func Logging(logger *log.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			scoped := logger.With("request_method", r.Method, "request_path", r.URL.Path)
			r = r.WithContext(logging.Context(r.Context(), scoped))

			next.ServeHTTP(w, r)

			scoped.Info("request", "duration", time.Since(start))
		})
	}
}
