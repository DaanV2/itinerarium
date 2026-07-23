package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
	"github.com/DaanV2/itinerarium/api/transport"
)

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
func LoginHandler(svc *application.AuthService, throttle *transport.Throttle, trustProxy bool) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		ipKey := "login:ip:" + transport.ClientIP(r, trustProxy)
		acctKey := "login:acct:" + strings.ToLower(strings.TrimSpace(req.Email))

		if ok, retry := throttle.Allowed(ipKey); !ok {
			transport.WriteThrottled(w, retry)

			return
		}

		if ok, retry := throttle.Allowed(acctKey); !ok {
			transport.WriteThrottled(w, retry)

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
