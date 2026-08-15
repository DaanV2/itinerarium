package locationsv1

import (
	"encoding/json"
	"fmt"
	"net/http"

	charactersv1 "github.com/DaanV2/itinerarium/api/api/v1/characters"
	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
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

type characterResponse struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	CurrentGameDay int     `json:"current_game_day"`
	UserID         string  `json:"user_id"`
	LocationID     *string `json:"location_id,omitempty"`
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

// Route paths for the location resource. LocationsPath / LocationPath are the
// shared bases that location subresource groups in other packages (inventory)
// build their own paths from.
const (
	LocationsPath      = "/api/locations"
	LocationPath       = LocationsPath + "/{id}"
	LocationAccessPath = LocationPath + "/access"
	LocationAccessID   = LocationAccessPath + "/{accessId}"
)

// LocationHandler serves locations and their access grants under
// /api/locations.
type LocationHandler struct {
	locations *application.LocationService
}

// NewLocationHandler builds the location resource handler.
func NewLocationHandler(locations *application.LocationService) *LocationHandler {
	return &LocationHandler{locations: locations}
}

// Register wires the location routes onto r. Each handler must be reached
// through RequireAuth.
func (h *LocationHandler) Register(r *transport.Router) {
	r.Handle("GET "+LocationsPath, h.List())
	r.Handle("POST "+LocationsPath, h.Create())
	r.Handle("GET "+LocationPath, h.Get())
	r.Handle("PATCH "+LocationPath, h.Update())
	r.Handle("GET "+LocationAccessPath, h.ListAccess())
	r.Handle("POST "+LocationAccessPath, h.GrantAccess())
	r.Handle("DELETE "+LocationAccessID, h.RevokeAccess())
}

// Create lets a GM create a location.
func (h *LocationHandler) Create() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req createLocationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		location, err := h.locations.Create(r.Context(), transport.RequesterFrom(r), req.Name, req.Plane)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toLocationResponse(location))
	})
}

// List returns every location a caller may see: all of them for a GM, only
// accessible ones for a player.
func (h *LocationHandler) List() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		locations, err := h.locations.List(r.Context(), transport.RequesterFrom(r))
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

// Get returns one location, or 404 when the caller may not see it (existence
// hidden).
func (h *LocationHandler) Get() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		location, err := h.locations.Get(r.Context(), transport.RequesterFrom(r), r.PathValue("id"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toLocationResponse(location))
	})
}

// Update edits a location — anyone who can see it can edit it.
func (h *LocationHandler) Update() http.Handler {
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

		location, err := h.locations.Update(
			r.Context(), transport.RequesterFrom(r), r.PathValue("id"), req.Name, req.Plane, req.SharedOnGameDay, sections,
		)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toLocationResponse(location))
	})
}

// GrantAccess lets a GM grant a character or group access to a location.
func (h *LocationHandler) GrantAccess() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req grantLocationAccessRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		grant, err := h.locations.GrantAccess(
			r.Context(), transport.RequesterFrom(r), r.PathValue("id"), req.CharacterID, req.GroupID,
		)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toLocationAccessResponse(grant))
	})
}

// ListAccess lets a GM list the grants on a location.
func (h *LocationHandler) ListAccess() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		grants, err := h.locations.ListAccess(r.Context(), transport.RequesterFrom(r), r.PathValue("id"))
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

// RevokeAccess lets a GM remove one grant from a location.
func (h *LocationHandler) RevokeAccess() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		err := h.locations.RevokeAccess(r.Context(), transport.RequesterFrom(r), r.PathValue("id"), r.PathValue("accessId"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// CharacterLocationPath is a character's location association, addressed by the
// character {id}.
const CharacterLocationPath = charactersv1.CharacterPath + "/location"

// CharacterLocationHandler serves a character's location association under
// /api/characters/{id}/location. It reuses the location service.
type CharacterLocationHandler struct {
	locations *application.LocationService
}

// NewCharacterLocationHandler builds the character-location handler.
func NewCharacterLocationHandler(locations *application.LocationService) *CharacterLocationHandler {
	return &CharacterLocationHandler{locations: locations}
}

// Register wires the character-location routes onto r. Each handler must be
// reached through RequireAuth.
func (h *CharacterLocationHandler) Register(r *transport.Router) {
	r.Handle("PUT "+CharacterLocationPath, h.Set())
	r.Handle("DELETE "+CharacterLocationPath, h.Clear())
}

// Set associates a character with a location.
func (h *CharacterLocationHandler) Set() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req setCharacterLocationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.LocationID == "" {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		character, err := h.locations.AssignCharacter(
			r.Context(), transport.RequesterFrom(r), r.PathValue("id"), &req.LocationID,
		)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toCharacterResponse(character))
	})
}

// Clear removes a character's location association.
func (h *CharacterLocationHandler) Clear() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		character, err := h.locations.AssignCharacter(r.Context(), transport.RequesterFrom(r), r.PathValue("id"), nil)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toCharacterResponse(character))
	})
}

func toCharacterResponse(c *models.Character) characterResponse {
	return characterResponse{
		ID: c.ID, Name: c.Name, CurrentGameDay: c.CurrentGameDay, UserID: c.UserID, LocationID: c.LocationID,
	}
}
