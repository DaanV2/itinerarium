package transport

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
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

// CreateJournalEntryHandler adds a journal entry to the character named by
// {id}, stamped with its current_game_day. Must be wrapped in RequireAuth.
func CreateJournalEntryHandler(svc *application.JournalEntryService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req createJournalEntryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		e, err := svc.Create(r.Context(), requesterFrom(r), r.PathValue("id"), req.Content)
		if err != nil {
			writeJournalEntryServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, toJournalEntryResponse(e))
	})
}

// ListJournalEntriesHandler returns every journal entry for the character
// named by {id}. Must be wrapped in RequireAuth.
func ListJournalEntriesHandler(svc *application.JournalEntryService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entries, err := svc.List(r.Context(), requesterFrom(r), r.PathValue("id"))
		if err != nil {
			writeJournalEntryServiceError(w, err)

			return
		}

		responses := make([]journalEntryResponse, len(entries))
		for i := range entries {
			responses[i] = toJournalEntryResponse(&entries[i])
		}

		writeJSON(w, http.StatusOK, responses)
	})
}

// GetJournalEntryHandler returns a single journal entry. Must be wrapped in
// RequireAuth.
func GetJournalEntryHandler(svc *application.JournalEntryService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e, err := svc.Get(r.Context(), requesterFrom(r), r.PathValue("entryId"))
		if err != nil {
			writeJournalEntryServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toJournalEntryResponse(e))
	})
}

// UpdateJournalEntryHandler edits a journal entry's content. Must be wrapped
// in RequireAuth.
func UpdateJournalEntryHandler(svc *application.JournalEntryService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req updateJournalEntryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		e, err := svc.Update(r.Context(), requesterFrom(r), r.PathValue("entryId"), req.Content)
		if err != nil {
			writeJournalEntryServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toJournalEntryResponse(e))
	})
}

func writeJournalEntryServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, application.ErrInvalidContent):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "processing request")
	}
}
