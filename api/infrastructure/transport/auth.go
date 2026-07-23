package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
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
// access token. No auth required. The throttle (nil disables it) rate-limits
// failed attempts per client IP and per account with exponential backoff, so an
// internet-facing deployment without a reverse proxy still can't be brute-forced
// (roadmap M10); trustProxy decides whether X-Forwarded-For names the client.
func LoginHandler(svc *application.AuthService, throttle *Throttle, trustProxy bool) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		ipKey := "login:ip:" + clientIP(r, trustProxy)
		acctKey := "login:acct:" + strings.ToLower(strings.TrimSpace(req.Email))

		if ok, retry := throttle.Allowed(ipKey); !ok {
			writeThrottled(w, retry)

			return
		}

		if ok, retry := throttle.Allowed(acctKey); !ok {
			writeThrottled(w, retry)

			return
		}

		user, token, err := svc.Login(r.Context(), req.Email, req.Password)
		if err != nil {
			// Count only genuine credential failures — a server error is not the
			// caller's fault and must not push them toward a lockout.
			if errors.Is(err, application.ErrInvalidCredentials) {
				throttle.Penalize(ipKey)
				throttle.Penalize(acctKey)
			}

			writeLoginError(w, err)

			return
		}

		// A success clears the account's own failures, but deliberately not the
		// IP's: on a shared/NAT address a legitimate login must not wipe an
		// attacker's accumulated penalty.
		throttle.Reset(acctKey)

		w.WriteJSON(http.StatusOK, loginResponse{
			ID: user.ID, Email: user.Email, Role: user.Role, AccessToken: token,
		})
	})
}

func writeLoginError(w xhttp.JSONResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrInvalidCredentials):
		w.WriteErrorMsg(http.StatusUnauthorized, "invalid credentials")
	default:
		w.WriteError(http.StatusInternalServerError, fmt.Errorf("logging in: %w", err))
	}
}

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

// requesterFrom extracts the Requester injected by RequireAuth. Handlers
// registered without RequireAuth must not call this.
func requesterFrom(r *http.Request) application.Requester {
	requester, _ := r.Context().Value(requesterContextKey).(application.Requester)

	return requester
}
