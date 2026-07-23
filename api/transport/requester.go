package transport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
)

type contextKey int

const requesterContextKey contextKey = iota

// RequireAuth validates the bearer token on the wrapped handler and injects
// the resulting Requester into the request context. Requests without a
// valid, unexpired, unrevoked token get 401 before the handler runs.
func RequireAuth(auth *application.AuthService) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := getBearerToken(r)
			if token == "" {
				xhttp.WriteError(w, http.StatusUnauthorized, errors.New("missing bearer token"))

				return
			}

			requester, err := auth.Authenticate(r.Context(), token)
			if err != nil {
				xhttp.WriteError(w, http.StatusUnauthorized, fmt.Errorf("invalid or expired token: %w", err))

				return
			}

			// Install the per-request gating cache so a single request resolves
			// the requester's characters and group memberships once (roadmap M8).
			ctx := application.WithRequestCache(r.Context())
			ctx = context.WithValue(ctx, requesterContextKey, requester)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// getBearerToken extracts the bearer token from the Authorization header, if present.
func getBearerToken(r *http.Request) string {
	const prefix = "Bearer "

	header := r.Header.Get("Authorization")

	return strings.TrimPrefix(header, prefix)
}

// RequesterFrom extracts the Requester injected by RequireAuth. Handlers
// registered without RequireAuth must not call this.
func RequesterFrom(r *http.Request) application.Requester {
	requester, _ := r.Context().Value(requesterContextKey).(application.Requester)

	return requester
}
