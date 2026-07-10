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

type grantLocationAccessRequest struct {
	CharacterID *string `json:"character_id,omitempty"`
	GroupID     *string `json:"group_id,omitempty"`
}

type locationAccessResponse struct {
	ID          string  `json:"id"`
	LocationID  string  `json:"location_id"`
	CharacterID *string `json:"character_id,omitempty"`
	GroupID     *string `json:"group_id,omitempty"`
}

func toLocationAccessResponse(a *models.LocationAccess) locationAccessResponse {
	return locationAccessResponse{
		ID: a.ID, LocationID: a.LocationID, CharacterID: a.CharacterID, GroupID: a.GroupID,
	}
}

type setCharacterLocationRequest struct {
	LocationID string `json:"location_id"`
}

// CreateLocationHandler lets a GM create a location. Must be wrapped in
// RequireAuth.
func CreateLocationHandler(svc *application.LocationService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req createLocationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		location, err := svc.Create(r.Context(), requesterFrom(r), req.Name, req.Description, req.Plane)
		if err != nil {
			writeLocationServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, toLocationResponse(location))
	})
}

// ListLocationsHandler returns every location a caller may see: all of them
// for a GM, only accessible ones for a player. Must be wrapped in RequireAuth.
func ListLocationsHandler(svc *application.LocationService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locations, err := svc.List(r.Context(), requesterFrom(r))
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

// GetLocationHandler returns one location, or 404 when the caller may not see
// it (existence hidden). Must be wrapped in RequireAuth.
func GetLocationHandler(svc *application.LocationService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		location, err := svc.Get(r.Context(), requesterFrom(r), r.PathValue("id"))
		if err != nil {
			writeLocationServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toLocationResponse(location))
	})
}

// UpdateLocationHandler edits a location — anyone who can see it can edit it.
// Must be wrapped in RequireAuth.
func UpdateLocationHandler(svc *application.LocationService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req updateLocationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		location, err := svc.Update(
			r.Context(), requesterFrom(r), r.PathValue("id"), req.Name, req.Description, req.Plane,
		)
		if err != nil {
			writeLocationServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toLocationResponse(location))
	})
}

// GrantLocationAccessHandler lets a GM grant a character or group access to a
// location. Must be wrapped in RequireAuth.
func GrantLocationAccessHandler(svc *application.LocationService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req grantLocationAccessRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		grant, err := svc.GrantAccess(
			r.Context(), requesterFrom(r), r.PathValue("id"), req.CharacterID, req.GroupID,
		)
		if err != nil {
			writeLocationServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, toLocationAccessResponse(grant))
	})
}

// ListLocationAccessHandler lets a GM list the grants on a location. Must be
// wrapped in RequireAuth.
func ListLocationAccessHandler(svc *application.LocationService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		grants, err := svc.ListAccess(r.Context(), requesterFrom(r), r.PathValue("id"))
		if err != nil {
			writeLocationServiceError(w, err)

			return
		}

		responses := make([]locationAccessResponse, len(grants))
		for i := range grants {
			responses[i] = toLocationAccessResponse(&grants[i])
		}

		writeJSON(w, http.StatusOK, responses)
	})
}

// RevokeLocationAccessHandler lets a GM remove one grant from a location.
// Must be wrapped in RequireAuth.
func RevokeLocationAccessHandler(svc *application.LocationService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := svc.RevokeAccess(r.Context(), requesterFrom(r), r.PathValue("id"), r.PathValue("accessId"))
		if err != nil {
			writeLocationServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// SetCharacterLocationHandler associates a character with a location. Must be
// wrapped in RequireAuth.
func SetCharacterLocationHandler(svc *application.LocationService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req setCharacterLocationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.LocationID == "" {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		character, err := svc.AssignCharacter(r.Context(), requesterFrom(r), r.PathValue("id"), &req.LocationID)
		if err != nil {
			writeLocationServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toCharacterResponse(character))
	})
}

// ClearCharacterLocationHandler removes a character's location association.
// Must be wrapped in RequireAuth.
func ClearCharacterLocationHandler(svc *application.LocationService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		character, err := svc.AssignCharacter(r.Context(), requesterFrom(r), r.PathValue("id"), nil)
		if err != nil {
			writeLocationServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toCharacterResponse(character))
	})
}

func writeLocationServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, application.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, application.ErrAlreadyGranted):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, application.ErrInvalidName), errors.Is(err, application.ErrInvalidGrant):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "processing request")
	}
}
