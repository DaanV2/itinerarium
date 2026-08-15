package sessionsv1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
)

type createSessionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type updateSessionRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type addSessionParticipantRequest struct {
	CharacterID string `json:"character_id"`
}

type advanceGameDayRequest struct {
	Delta       int     `json:"delta"`
	CharacterID *string `json:"character_id,omitempty"`
}

// sessionParticipantResponse deliberately exposes only a participant's
// identity, matching the group member response.
type sessionParticipantResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type sessionResponse struct {
	ID           string                       `json:"id"`
	Name         string                       `json:"name"`
	Description  string                       `json:"description,omitempty"`
	Participants []sessionParticipantResponse `json:"participants"`
}

func toSessionResponse(s *models.Session) sessionResponse {
	participants := make([]sessionParticipantResponse, len(s.Participants))
	for i := range s.Participants {
		participants[i] = sessionParticipantResponse{ID: s.Participants[i].ID, Name: s.Participants[i].Name}
	}

	return sessionResponse{
		ID: s.ID, Name: s.Name, Description: s.Description, Participants: participants,
	}
}

// Route paths for the session resource.
const (
	SessionsPath            = "/api/sessions"
	SessionPath             = SessionsPath + "/{id}"
	SessionParticipantsPath = SessionPath + "/participants"
	SessionParticipantPath  = SessionParticipantsPath + "/{characterId}"
	SessionGameDayPath      = SessionPath + "/game-day"
)

// SessionHandler serves sessions, participants, and game-day advances under
// /api/sessions.
type SessionHandler struct {
	sessions *application.SessionService
}

// NewSessionHandler builds the session resource handler.
func NewSessionHandler(sessions *application.SessionService) *SessionHandler {
	return &SessionHandler{sessions: sessions}
}

// Register wires the session routes onto r. Each handler must be reached
// through RequireAuth.
func (h *SessionHandler) Register(r *transport.Router) {
	r.Handle("GET "+SessionsPath, h.List())
	r.Handle("POST "+SessionsPath, h.Create())
	r.Handle("GET "+SessionPath, h.Get())
	r.Handle("PATCH "+SessionPath, h.Update())
	r.Handle("POST "+SessionParticipantsPath, h.AddParticipant())
	r.Handle("DELETE "+SessionParticipantPath, h.RemoveParticipant())
	r.Handle("POST "+SessionGameDayPath, h.AdvanceGameDay())
}

// Create lets a GM create a session.
func (h *SessionHandler) Create() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req createSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		session, err := h.sessions.Create(r.Context(), transport.RequesterFrom(r), req.Name, req.Description)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toSessionResponse(session))
	})
}

// List returns every session with its participants.
func (h *SessionHandler) List() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		sessions, err := h.sessions.List(r.Context(), transport.RequesterFrom(r))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		responses := make([]sessionResponse, len(sessions))
		for i := range sessions {
			responses[i] = toSessionResponse(&sessions[i])
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

// Get returns one session with its participants.
func (h *SessionHandler) Get() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		session, err := h.sessions.Get(r.Context(), transport.RequesterFrom(r), r.PathValue("id"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toSessionResponse(session))
	})
}

// Update lets a GM edit a session's name or description.
func (h *SessionHandler) Update() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req updateSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		session, err := h.sessions.Update(
			r.Context(), transport.RequesterFrom(r), r.PathValue("id"), req.Name, req.Description,
		)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toSessionResponse(session))
	})
}

// AddParticipant lets a GM add a character to a session.
func (h *SessionHandler) AddParticipant() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req addSessionParticipantRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		err := h.sessions.AddParticipant(r.Context(), transport.RequesterFrom(r), r.PathValue("id"), req.CharacterID)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// RemoveParticipant lets a GM remove a character from a session.
func (h *SessionHandler) RemoveParticipant() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		err := h.sessions.RemoveParticipant(
			r.Context(), transport.RequesterFrom(r), r.PathValue("id"), r.PathValue("characterId"),
		)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// AdvanceGameDay lets a GM move game day forward or back for every session
// participant, or for one participant catching up.
func (h *SessionHandler) AdvanceGameDay() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req advanceGameDayRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		session, err := h.sessions.AdvanceGameDay(
			r.Context(), transport.RequesterFrom(r), r.PathValue("id"), req.Delta, req.CharacterID,
		)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toSessionResponse(session))
	})
}
