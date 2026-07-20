package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
)

type contextKey int

const requesterContextKey contextKey = iota

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	ID          string      `json:"id"`
	Email       string      `json:"email"`
	Role        models.Role `json:"role"`
	AccessToken string      `json:"access_token"`
}

// LoginHandler authenticates an email + password pair and returns a signed
// access token. No auth required.
func LoginHandler(svc *application.AuthService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		user, token, err := svc.Login(r.Context(), req.Email, req.Password)
		if err != nil {
			writeLoginError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, loginResponse{
			ID: user.ID, Email: user.Email, Role: user.Role, AccessToken: token,
		})
	})
}

func writeLoginError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "logging in")
	}
}

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

			// Install the per-request gating cache so a single request resolves
			// the requester's characters and group memberships once (roadmap M8).
			ctx := application.WithRequestCache(r.Context())
			ctx = context.WithValue(ctx, requesterContextKey, requester)
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
