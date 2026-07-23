package transport

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
)

type createAccountRequest struct {
	Email string      `json:"email"`
	Role  models.Role `json:"role"`
}

type accountResponse struct {
	ID                string      `json:"id"`
	Email             string      `json:"email"`
	Role              models.Role `json:"role"`
	TemporaryPassword string      `json:"temporary_password,omitempty"`
}

type resetPasswordResponse struct {
	TemporaryPassword string `json:"temporary_password"`
}

// CreateAccountHandler lets a GM create a new player or GM account, handing
// back a random temporary password for the GM to relay out of band. Must be
// wrapped in RequireAuth.
func CreateAccountHandler(svc *application.UserService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req createAccountRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		user, password, err := svc.CreateAccount(r.Context(), requesterFrom(r), req.Email, req.Role)
		if err != nil {
			writeServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, accountResponse{
			ID: user.ID, Email: user.Email, Role: user.Role, TemporaryPassword: password,
		})
	})
}

// ListAccountsHandler lets a GM list every account for the admin panel. Must
// be wrapped in RequireAuth.
func ListAccountsHandler(svc *application.UserService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		users, err := svc.ListAccounts(r.Context(), requesterFrom(r))
		if err != nil {
			writeServiceError(w, err)

			return
		}

		accounts := make([]accountResponse, len(users))
		for i := range users {
			accounts[i] = accountResponse{ID: users[i].ID, Email: users[i].Email, Role: users[i].Role}
		}

		w.WriteJSON(http.StatusOK, accounts)
	})
}

// ResetPasswordHandler lets a GM reset another account's password to a fresh
// random temporary password, handed back for the GM to relay out of band. No
// SMTP dependency. Must be wrapped in RequireAuth. The throttle (nil disables
// it) caps repeated resets against a single target account (roadmap M10) — this
// path is authenticated and GM-only, so account spam, not credential guessing,
// is the risk, hence keying by target rather than IP.
func ResetPasswordHandler(svc *application.UserService, throttle *Throttle) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		targetID := r.PathValue("id")
		key := "reset:acct:" + targetID

		if ok, retry := throttle.Allowed(key); !ok {
			writeThrottled(w, retry)

			return
		}

		password, err := svc.ResetPassword(r.Context(), requesterFrom(r), targetID)
		if err != nil {
			writeServiceError(w, err)

			return
		}

		// Every successful reset is chargeable: there is no "good" outcome that
		// should clear the counter, so the decay window caps resets per window.
		throttle.Penalize(key)

		w.WriteJSON(http.StatusOK, resetPasswordResponse{TemporaryPassword: password})
	})
}
