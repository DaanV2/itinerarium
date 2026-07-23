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

type createCharacterRequest struct {
	Name   string `json:"name"`
	UserID string `json:"user_id,omitempty"`
}

type updateCharacterRequest struct {
	Name           *string `json:"name,omitempty"`
	CurrentGameDay *int    `json:"current_game_day,omitempty"`
}

type characterResponse struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	CurrentGameDay int     `json:"current_game_day"`
	UserID         string  `json:"user_id"`
	LocationID     *string `json:"location_id,omitempty"`
}

func toCharacterResponse(c *models.Character) characterResponse {
	return characterResponse{
		ID: c.ID, Name: c.Name, CurrentGameDay: c.CurrentGameDay, UserID: c.UserID, LocationID: c.LocationID,
	}
}

// CreateCharacterHandler lets a caller create a character for themselves, or
// a GM create one for any existing user. Must be wrapped in RequireAuth.
func CreateCharacterHandler(svc *application.CharacterService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req createCharacterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		c, err := svc.Create(r.Context(), transport.RequesterFrom(r), req.UserID, req.Name)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toCharacterResponse(c))
	})
}

// ListCharactersHandler returns the caller's own characters, or every
// character for a GM. Must be wrapped in RequireAuth.
func ListCharactersHandler(svc *application.CharacterService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		characters, err := svc.List(r.Context(), transport.RequesterFrom(r))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		responses := make([]characterResponse, len(characters))
		for i := range characters {
			responses[i] = toCharacterResponse(&characters[i])
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

// GetCharacterHandler returns a single character owned by the caller, or any
// character for a GM. Must be wrapped in RequireAuth.
func GetCharacterHandler(svc *application.CharacterService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		c, err := svc.Get(r.Context(), transport.RequesterFrom(r), r.PathValue("id"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toCharacterResponse(c))
	})
}

// UpdateCharacterHandler renames a character and/or (GM only) sets its
// current_game_day. Must be wrapped in RequireAuth.
func UpdateCharacterHandler(svc *application.CharacterService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req updateCharacterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		c, err := svc.Update(r.Context(), transport.RequesterFrom(r), r.PathValue("id"), req.Name, req.CurrentGameDay)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toCharacterResponse(c))
	})
}
