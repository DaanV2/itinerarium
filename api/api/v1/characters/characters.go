package charactersv1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
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

// Route paths for the character resource. CharactersPath / CharacterPath are
// the shared bases that character subresource groups in other packages
// (journal, inventory, money, location, activity) build their own paths from.
const (
	CharactersPath = "/api/characters"
	CharacterPath  = CharactersPath + "/{id}"
)

// CharacterHandler serves the character resource under /api/characters.
type CharacterHandler struct {
	characters *application.CharacterService
}

// NewCharacterHandler builds the character resource handler.
func NewCharacterHandler(characters *application.CharacterService) *CharacterHandler {
	return &CharacterHandler{characters: characters}
}

// Register wires the character routes onto r. Each handler must be reached
// through RequireAuth.
func (h *CharacterHandler) Register(r *transport.Router) {
	r.Handle("GET "+CharactersPath, h.List())
	r.Handle("POST "+CharactersPath, h.Create())
	r.Handle("GET "+CharacterPath, h.Get())
	r.Handle("PATCH "+CharacterPath, h.Update())
}

// Create lets a caller create a character for themselves, or a GM create one
// for any existing user.
func (h *CharacterHandler) Create() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req createCharacterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		c, err := h.characters.Create(r.Context(), transport.RequesterFrom(r), req.UserID, req.Name)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toCharacterResponse(c))
	})
}

// List returns the caller's own characters, or every character for a GM.
func (h *CharacterHandler) List() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		characters, err := h.characters.List(r.Context(), transport.RequesterFrom(r))
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

// Get returns a single character owned by the caller, or any character for a
// GM.
func (h *CharacterHandler) Get() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		c, err := h.characters.Get(r.Context(), transport.RequesterFrom(r), r.PathValue("id"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toCharacterResponse(c))
	})
}

// Update renames a character and/or (GM only) sets its current_game_day.
func (h *CharacterHandler) Update() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req updateCharacterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		c, err := h.characters.Update(
			r.Context(), transport.RequesterFrom(r), r.PathValue("id"), req.Name, req.CurrentGameDay,
		)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toCharacterResponse(c))
	})
}
