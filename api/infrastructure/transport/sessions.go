package transport

import (
	"encoding/json"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
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

// CreateSessionHandler lets a GM create a session. Must be wrapped in
// RequireAuth.
func CreateSessionHandler(svc *application.SessionService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req createSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		session, err := svc.Create(r.Context(), requesterFrom(r), req.Name, req.Description)
		if err != nil {
			writeServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, toSessionResponse(session))
	})
}

// ListSessionsHandler returns every session with its participants. Must be
// wrapped in RequireAuth.
func ListSessionsHandler(svc *application.SessionService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessions, err := svc.List(r.Context(), requesterFrom(r))
		if err != nil {
			writeServiceError(w, err)

			return
		}

		responses := make([]sessionResponse, len(sessions))
		for i := range sessions {
			responses[i] = toSessionResponse(&sessions[i])
		}

		writeJSON(w, http.StatusOK, responses)
	})
}

// GetSessionHandler returns one session with its participants. Must be
// wrapped in RequireAuth.
func GetSessionHandler(svc *application.SessionService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := svc.Get(r.Context(), requesterFrom(r), r.PathValue("id"))
		if err != nil {
			writeServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toSessionResponse(session))
	})
}

// UpdateSessionHandler lets a GM edit a session's name or description. Must
// be wrapped in RequireAuth.
func UpdateSessionHandler(svc *application.SessionService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req updateSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		session, err := svc.Update(r.Context(), requesterFrom(r), r.PathValue("id"), req.Name, req.Description)
		if err != nil {
			writeServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toSessionResponse(session))
	})
}

// AddSessionParticipantHandler lets a GM add a character to a session. Must
// be wrapped in RequireAuth.
func AddSessionParticipantHandler(svc *application.SessionService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req addSessionParticipantRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		err := svc.AddParticipant(r.Context(), requesterFrom(r), r.PathValue("id"), req.CharacterID)
		if err != nil {
			writeServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// RemoveSessionParticipantHandler lets a GM remove a character from a
// session. Must be wrapped in RequireAuth.
func RemoveSessionParticipantHandler(svc *application.SessionService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := svc.RemoveParticipant(r.Context(), requesterFrom(r), r.PathValue("id"), r.PathValue("characterId"))
		if err != nil {
			writeServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// AdvanceSessionGameDayHandler lets a GM move game day forward or back for
// every session participant, or for one participant catching up. Must be
// wrapped in RequireAuth.
func AdvanceSessionGameDayHandler(svc *application.SessionService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req advanceGameDayRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		session, err := svc.AdvanceGameDay(r.Context(), requesterFrom(r), r.PathValue("id"), req.Delta, req.CharacterID)
		if err != nil {
			writeServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toSessionResponse(session))
	})
}
