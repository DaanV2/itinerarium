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
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req createJournalEntryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		e, err := svc.Create(r.Context(), transport.RequesterFrom(r), r.PathValue("id"), req.Content)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toJournalEntryResponse(e))
	})
}

// ListJournalEntriesHandler returns every journal entry for the character
// named by {id}. Must be wrapped in RequireAuth.
func ListJournalEntriesHandler(svc *application.JournalEntryService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		entries, err := svc.List(r.Context(), transport.RequesterFrom(r), r.PathValue("id"))
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

// GetJournalEntryHandler returns a single journal entry. Must be wrapped in
// RequireAuth.
func GetJournalEntryHandler(svc *application.JournalEntryService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		e, err := svc.Get(r.Context(), transport.RequesterFrom(r), r.PathValue("entryId"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toJournalEntryResponse(e))
	})
}

// UpdateJournalEntryHandler edits a journal entry's content. Must be wrapped
// in RequireAuth.
func UpdateJournalEntryHandler(svc *application.JournalEntryService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req updateJournalEntryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		e, err := svc.Update(r.Context(), transport.RequesterFrom(r), r.PathValue("entryId"), req.Content)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toJournalEntryResponse(e))
	})
}

// ConvertJournalEntryHandler copies a journal entry into a new document in
// the character's personal repository. The journal entry itself is left
// untouched. Must be wrapped in RequireAuth.
func ConvertJournalEntryHandler(svc *application.JournalEntryService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		view, err := svc.Convert(r.Context(), transport.RequesterFrom(r), r.PathValue("entryId"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toDocumentResponse(view))
	})
}
