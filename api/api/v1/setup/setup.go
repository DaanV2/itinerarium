package setupv1

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
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

// SetupPath is the first-run wizard endpoint (public — no auth).
const SetupPath = "/api/setup"

// SetupHandler serves the first-run wizard under /api/setup. These routes are
// public: they run before any account exists.
type SetupHandler struct {
	setup *application.SetupService
}

// NewSetupHandler builds the first-run wizard handler.
func NewSetupHandler(setup *application.SetupService) *SetupHandler {
	return &SetupHandler{setup: setup}
}

// Register wires the setup routes onto r. These handlers are public.
func (h *SetupHandler) Register(r *transport.Router) {
	r.Handle("GET "+SetupPath, h.Status())
	r.Handle("POST "+SetupPath, h.CreateInitialGM())
}

// Status reports whether the first-run wizard still needs to run.
func (h *SetupHandler) Status() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		needsSetup, err := h.setup.NeedsSetup(r.Context())
		if err != nil {
			w.WriteError(http.StatusInternalServerError, fmt.Errorf("checking setup status: %w", err))

			return
		}

		w.WriteJSON(http.StatusOK, setupStatusResponse{NeedsSetup: needsSetup})
	})
}

// CreateInitialGM runs the first-run wizard, creating the installation's sole
// initial GM account. It refuses once any account exists.
func (h *SetupHandler) CreateInitialGM() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req setupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		user, token, err := h.setup.CreateInitialGM(r.Context(), req.Email, req.Password)
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
