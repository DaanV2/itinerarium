package transport

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
)

type createLocationRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Plane       string `json:"plane,omitempty"`
}

type updateLocationRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Plane       *string `json:"plane,omitempty"`
}

type locationResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Plane       string `json:"plane,omitempty"`
}

func toLocationResponse(l *models.Location) locationResponse {
	return locationResponse{ID: l.ID, Name: l.Name, Description: l.Description, Plane: l.Plane}
}

// ListLocationsHandler returns every location. Must be wrapped in RequireAuth.
func ListLocationsHandler(svc *application.LocationService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locations, err := svc.List(r.Context())
		if err != nil {
			writeLocationServiceError(w, err)

			return
		}

		responses := make([]locationResponse, len(locations))
		for i := range locations {
			responses[i] = toLocationResponse(&locations[i])
		}

		writeJSON(w, http.StatusOK, responses)
	})
}

// GetLocationHandler returns a single location. Must be wrapped in RequireAuth.
func GetLocationHandler(svc *application.LocationService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l, err := svc.Get(r.Context(), requesterFrom(r), r.PathValue("id"))
		if err != nil {
			writeLocationServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toLocationResponse(l))
	})
}

// CreateLocationHandler lets a GM add a location. Must be wrapped in
// RequireAuth.
func CreateLocationHandler(svc *application.LocationService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req createLocationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		l, err := svc.Create(r.Context(), requesterFrom(r), req.Name, req.Description, req.Plane)
		if err != nil {
			writeLocationServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, toLocationResponse(l))
	})
}

// UpdateLocationHandler lets a GM edit a location. Must be wrapped in
// RequireAuth.
func UpdateLocationHandler(svc *application.LocationService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req updateLocationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		l, err := svc.Update(r.Context(), requesterFrom(r), r.PathValue("id"), req.Name, req.Description, req.Plane)
		if err != nil {
			writeLocationServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toLocationResponse(l))
	})
}

func writeLocationServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, application.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, application.ErrInvalidName):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "processing request")
	}
}
