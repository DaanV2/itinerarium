package knowledgev1

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

type createJournalEntryRequest struct {
	Content string `json:"content"`
}

type updateJournalEntryRequest struct {
	Content string `json:"content"`
}

type journalEntryResponse struct {
	ID          string `json:"id"`
	CharacterID string `json:"character_id"`
	GameDay     int    `json:"game_day"`
	Content     string `json:"content"`
}

func toJournalEntryResponse(e *models.JournalEntry) journalEntryResponse {
	return journalEntryResponse{ID: e.ID, CharacterID: e.CharacterID, GameDay: e.GameDay, Content: e.Content}
}

// Route paths for a character's journal, addressed by the character {id}.
const (
	CharacterJournalBasePath    = charactersv1.CharacterPath + "/journal"
	CharacterJournalEntryPath   = CharacterJournalBasePath + "/{entryId}"
	CharacterJournalConvertPath = CharacterJournalEntryPath + "/convert"
)

// CharacterJournalHandler serves one character's journal entries under
// /api/characters/{id}/journal. It reuses the journal service.
type CharacterJournalHandler struct {
	journals *application.JournalEntryService
}

// NewCharacterJournalHandler builds the character-journal handler.
func NewCharacterJournalHandler(journals *application.JournalEntryService) *CharacterJournalHandler {
	return &CharacterJournalHandler{journals: journals}
}

// Register wires the character-journal routes onto r. Each handler must be
// reached through RequireAuth.
func (h *CharacterJournalHandler) Register(r *transport.Router) {
	r.Handle("GET "+CharacterJournalBasePath, h.List())
	r.Handle("POST "+CharacterJournalBasePath, h.Create())
	r.Handle("GET "+CharacterJournalEntryPath, h.Get())
	r.Handle("PATCH "+CharacterJournalEntryPath, h.Update())
	r.Handle("POST "+CharacterJournalConvertPath, h.Convert())
}

// Create adds a journal entry to the character named by {id}, stamped with its
// current_game_day.
func (h *CharacterJournalHandler) Create() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req createJournalEntryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		e, err := h.journals.Create(r.Context(), transport.RequesterFrom(r), r.PathValue("id"), req.Content)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toJournalEntryResponse(e))
	})
}

// List returns every journal entry for the character named by {id}.
func (h *CharacterJournalHandler) List() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		entries, err := h.journals.List(r.Context(), transport.RequesterFrom(r), r.PathValue("id"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		responses := make([]journalEntryResponse, len(entries))
		for i := range entries {
			responses[i] = toJournalEntryResponse(&entries[i])
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

// Get returns a single journal entry.
func (h *CharacterJournalHandler) Get() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		e, err := h.journals.Get(r.Context(), transport.RequesterFrom(r), r.PathValue("entryId"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toJournalEntryResponse(e))
	})
}

// Update edits a journal entry's content.
func (h *CharacterJournalHandler) Update() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req updateJournalEntryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		e, err := h.journals.Update(r.Context(), transport.RequesterFrom(r), r.PathValue("entryId"), req.Content)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toJournalEntryResponse(e))
	})
}

// Convert copies a journal entry into a new document in the character's
// personal repository. The journal entry itself is left untouched.
func (h *CharacterJournalHandler) Convert() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		view, err := h.journals.Convert(r.Context(), transport.RequesterFrom(r), r.PathValue("entryId"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toDocumentResponse(view))
	})
}
