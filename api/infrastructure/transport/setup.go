package transport

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
)

type setupStatusResponse struct {
	NeedsSetup bool `json:"needs_setup"`
}

type setupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type setupResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	AccessToken string `json:"access_token"`
}

// SetupStatusHandler reports whether the first-run wizard still needs to run.
func SetupStatusHandler(svc *application.SetupService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		needsSetup, err := svc.NeedsSetup(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "checking setup status")

			return
		}

		writeJSON(w, http.StatusOK, setupStatusResponse{NeedsSetup: needsSetup})
	})
}

// CreateInitialGMHandler runs the first-run wizard, creating the
// installation's sole initial GM account. It refuses once any account
// exists.
func CreateInitialGMHandler(svc *application.SetupService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req setupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		user, token, err := svc.CreateInitialGM(r.Context(), req.Email, req.Password)
		if err != nil {
			writeSetupError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, setupResponse{ID: user.ID, Email: user.Email, AccessToken: token})
	})
}

func writeSetupError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrAlreadySetUp):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, application.ErrInvalidEmail), errors.Is(err, application.ErrInvalidPassword):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "creating account")
	}
}
