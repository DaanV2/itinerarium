package transport

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req createCharacterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		c, err := svc.Create(r.Context(), requesterFrom(r), req.UserID, req.Name)
		if err != nil {
			writeCharacterServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, toCharacterResponse(c))
	})
}

// ListCharactersHandler returns the caller's own characters, or every
// character for a GM. Must be wrapped in RequireAuth.
func ListCharactersHandler(svc *application.CharacterService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		characters, err := svc.List(r.Context(), requesterFrom(r))
		if err != nil {
			writeCharacterServiceError(w, err)

			return
		}

		responses := make([]characterResponse, len(characters))
		for i := range characters {
			responses[i] = toCharacterResponse(&characters[i])
		}

		writeJSON(w, http.StatusOK, responses)
	})
}

// GetCharacterHandler returns a single character owned by the caller, or any
// character for a GM. Must be wrapped in RequireAuth.
func GetCharacterHandler(svc *application.CharacterService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := svc.Get(r.Context(), requesterFrom(r), r.PathValue("id"))
		if err != nil {
			writeCharacterServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toCharacterResponse(c))
	})
}

// UpdateCharacterHandler renames a character and/or (GM only) sets its
// current_game_day. Must be wrapped in RequireAuth.
func UpdateCharacterHandler(svc *application.CharacterService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req updateCharacterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		c, err := svc.Update(r.Context(), requesterFrom(r), r.PathValue("id"), req.Name, req.CurrentGameDay)
		if err != nil {
			writeCharacterServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toCharacterResponse(c))
	})
}

func writeCharacterServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, application.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, application.ErrInvalidName), errors.Is(err, application.ErrInvalidGameDay):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "processing request")
	}
}
