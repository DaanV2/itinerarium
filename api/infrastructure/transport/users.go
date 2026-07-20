package transport

import (
	"encoding/json"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req createAccountRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		user, password, err := svc.CreateAccount(r.Context(), requesterFrom(r), req.Email, req.Role)
		if err != nil {
			writeServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, accountResponse{
			ID: user.ID, Email: user.Email, Role: user.Role, TemporaryPassword: password,
		})
	})
}

// ListAccountsHandler lets a GM list every account for the admin panel. Must
// be wrapped in RequireAuth.
func ListAccountsHandler(svc *application.UserService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		users, err := svc.ListAccounts(r.Context(), requesterFrom(r))
		if err != nil {
			writeServiceError(w, err)

			return
		}

		accounts := make([]accountResponse, len(users))
		for i := range users {
			accounts[i] = accountResponse{ID: users[i].ID, Email: users[i].Email, Role: users[i].Role}
		}

		writeJSON(w, http.StatusOK, accounts)
	})
}

// ResetPasswordHandler lets a GM reset another account's password to a fresh
// random temporary password, handed back for the GM to relay out of band. No
// SMTP dependency. Must be wrapped in RequireAuth.
func ResetPasswordHandler(svc *application.UserService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		password, err := svc.ResetPassword(r.Context(), requesterFrom(r), r.PathValue("id"))
		if err != nil {
			writeServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, resetPasswordResponse{TemporaryPassword: password})
	})
}
