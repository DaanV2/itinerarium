package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
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
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		needsSetup, err := svc.NeedsSetup(r.Context())
		if err != nil {
			w.WriteError(http.StatusInternalServerError, fmt.Errorf("checking setup status: %w", err))

			return
		}

		w.WriteJSON(http.StatusOK, setupStatusResponse{NeedsSetup: needsSetup})
	})
}

// CreateInitialGMHandler runs the first-run wizard, creating the
// installation's sole initial GM account. It refuses once any account
// exists.
func CreateInitialGMHandler(svc *application.SetupService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req setupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		user, token, err := svc.CreateInitialGM(r.Context(), req.Email, req.Password)
		if err != nil {
			writeSetupError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, setupResponse{ID: user.ID, Email: user.Email, AccessToken: token})
	})
}

func writeSetupError(w xhttp.JSONResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrAlreadySetUp):
		w.WriteError(http.StatusConflict, fmt.Errorf("setup already complete: %w", err))
	case errors.Is(err, application.ErrInvalidEmail), errors.Is(err, application.ErrInvalidPassword):
		w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request: %w", err))
	default:
		w.WriteError(http.StatusInternalServerError, fmt.Errorf("creating account: %w", err))
	}
}
