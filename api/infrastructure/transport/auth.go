package transport

import (
	"context"
	"net/http"
	"strings"

	"github.com/DaanV2/itinerarium/api/application"
)

type contextKey int

const requesterContextKey contextKey = iota

// RequireAuth validates the bearer token on the wrapped handler and injects
// the resulting Requester into the request context. Requests without a
// valid, unexpired, unrevoked token get 401 before the handler runs.
func RequireAuth(auth *application.AuthService) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" {
				writeError(w, http.StatusUnauthorized, "missing bearer token")

				return
			}

			requester, err := auth.Authenticate(r.Context(), token)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid or expired token")

				return
			}

			ctx := context.WithValue(r.Context(), requesterContextKey, requester)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "

	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return ""
	}

	return strings.TrimPrefix(header, prefix)
}

// requesterFrom extracts the Requester injected by RequireAuth. Handlers
// registered without RequireAuth must not call this.
func requesterFrom(r *http.Request) application.Requester {
	requester, _ := r.Context().Value(requesterContextKey).(application.Requester)

	return requester
}
