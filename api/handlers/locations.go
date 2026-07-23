package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
	"github.com/DaanV2/itinerarium/api/transport"
)

type createLocationRequest struct {
	Name  string `json:"name"`
	Plane string `json:"plane,omitempty"`
}

type locationSectionInput struct {
	ID      string `json:"id,omitempty"`
	Content string `json:"content"`
	GMOnly  bool   `json:"gm_only,omitempty"`
}

type updateLocationRequest struct {
	Name            *string                `json:"name,omitempty"`
	Plane           *string                `json:"plane,omitempty"`
	SharedOnGameDay *int                   `json:"shared_on_game_day,omitempty"`
	Sections        []locationSectionInput `json:"sections"`
}

type locationSectionResponse struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	GMOnly  bool   `json:"gm_only"`
}

type locationResponse struct {
	ID              string                    `json:"id"`
	Name            string                    `json:"name"`
	Plane           string                    `json:"plane,omitempty"`
	SharedOnGameDay int                       `json:"shared_on_game_day"`
	Revealed        bool                      `json:"revealed"`
	Sections        []locationSectionResponse `json:"sections"`
}

type locationSummaryResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Plane string `json:"plane,omitempty"`
}

func toLocationResponse(v *application.LocationView) locationResponse {
	sections := make([]locationSectionResponse, len(v.Location.Sections))
	for i := range v.Location.Sections {
		sections[i] = locationSectionResponse{
			ID: v.Location.Sections[i].ID, Content: v.Location.Sections[i].Content, GMOnly: v.Location.Sections[i].GMOnly,
		}
	}

	return locationResponse{
		ID: v.Location.ID, Name: v.Location.Name, Plane: v.Location.Plane,
		SharedOnGameDay: v.Location.SharedOnGameDay, Revealed: v.Revealed, Sections: sections,
	}
}

func toLocationSummaryResponse(l *models.Location) locationSummaryResponse {
	return locationSummaryResponse{ID: l.ID, Name: l.Name, Plane: l.Plane}
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
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req createLocationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		location, err := svc.Create(r.Context(), transport.RequesterFrom(r), req.Name, req.Plane)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toLocationResponse(location))
	})
}

// ListLocationsHandler returns every location a caller may see: all of them
// for a GM, only accessible ones for a player. Must be wrapped in RequireAuth.
func ListLocationsHandler(svc *application.LocationService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		locations, err := svc.List(r.Context(), transport.RequesterFrom(r))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		responses := make([]locationSummaryResponse, len(locations))
		for i := range locations {
			responses[i] = toLocationSummaryResponse(&locations[i])
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

// GetLocationHandler returns one location, or 404 when the caller may not see
// it (existence hidden). Must be wrapped in RequireAuth.
func GetLocationHandler(svc *application.LocationService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		location, err := svc.Get(r.Context(), transport.RequesterFrom(r), r.PathValue("id"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toLocationResponse(location))
	})
}

// UpdateLocationHandler edits a location — anyone who can see it can edit it.
// Must be wrapped in RequireAuth.
func UpdateLocationHandler(svc *application.LocationService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req updateLocationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		var sections []application.LocationSectionInput
		if req.Sections != nil {
			sections = make([]application.LocationSectionInput, len(req.Sections))
			for i := range req.Sections {
				sections[i] = application.LocationSectionInput{
					ID: req.Sections[i].ID, Content: req.Sections[i].Content, GMOnly: req.Sections[i].GMOnly,
				}
			}
		}

		location, err := svc.Update(
			r.Context(), transport.RequesterFrom(r), r.PathValue("id"), req.Name, req.Plane, req.SharedOnGameDay, sections,
		)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toLocationResponse(location))
	})
}

// GrantLocationAccessHandler lets a GM grant a character or group access to a
// location. Must be wrapped in RequireAuth.
func GrantLocationAccessHandler(svc *application.LocationService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req grantLocationAccessRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		grant, err := svc.GrantAccess(
			r.Context(), transport.RequesterFrom(r), r.PathValue("id"), req.CharacterID, req.GroupID,
		)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toLocationAccessResponse(grant))
	})
}

// ListLocationAccessHandler lets a GM list the grants on a location. Must be
// wrapped in RequireAuth.
func ListLocationAccessHandler(svc *application.LocationService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		grants, err := svc.ListAccess(r.Context(), transport.RequesterFrom(r), r.PathValue("id"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		responses := make([]locationAccessResponse, len(grants))
		for i := range grants {
			responses[i] = toLocationAccessResponse(&grants[i])
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

// RevokeLocationAccessHandler lets a GM remove one grant from a location.
// Must be wrapped in RequireAuth.
func RevokeLocationAccessHandler(svc *application.LocationService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		err := svc.RevokeAccess(r.Context(), transport.RequesterFrom(r), r.PathValue("id"), r.PathValue("accessId"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// SetCharacterLocationHandler associates a character with a location. Must be
// wrapped in RequireAuth.
func SetCharacterLocationHandler(svc *application.LocationService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req setCharacterLocationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.LocationID == "" {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		character, err := svc.AssignCharacter(r.Context(), transport.RequesterFrom(r), r.PathValue("id"), &req.LocationID)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toCharacterResponse(character))
	})
}

// ClearCharacterLocationHandler removes a character's location association.
// Must be wrapped in RequireAuth.
func ClearCharacterLocationHandler(svc *application.LocationService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		character, err := svc.AssignCharacter(r.Context(), transport.RequesterFrom(r), r.PathValue("id"), nil)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toCharacterResponse(character))
	})
}
