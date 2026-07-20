package transport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
)

type activityTargetResponse struct {
	CharacterID *string `json:"character_id,omitempty"`
	GroupID     *string `json:"group_id,omitempty"`
}

type activityEntryResponse struct {
	ID              string                   `json:"id"`
	GameDay         int                      `json:"game_day"`
	Action          models.ActivityAction    `json:"action"`
	EntityType      string                   `json:"entity_type,omitempty"`
	EntityID        string                   `json:"entity_id,omitempty"`
	EntityName      string                   `json:"entity_name"`
	Actor           string                   `json:"actor,omitempty"`
	CharacterID     string                   `json:"character_id,omitempty"`
	ScopeType       string                   `json:"scope_type,omitempty"`
	ScopeID         string                   `json:"scope_id,omitempty"`
	Announced       bool                     `json:"announced"`
	AnnouncedPublic bool                     `json:"announced_public,omitempty"`
	Targets         []activityTargetResponse `json:"targets,omitempty"`
	CreatedAt       time.Time                `json:"created_at"`
}

func toActivityEntryResponse(e *models.ActivityEntry) activityEntryResponse {
	targets := make([]activityTargetResponse, 0, len(e.Targets))
	for i := range e.Targets {
		targets = append(targets, activityTargetResponse{
			CharacterID: e.Targets[i].CharacterID,
			GroupID:     e.Targets[i].GroupID,
		})
	}

	return activityEntryResponse{
		ID:              e.ID,
		GameDay:         e.GameDay,
		Action:          e.Action,
		EntityType:      e.EntityType,
		EntityID:        e.EntityID,
		EntityName:      e.EntityName,
		Actor:           e.Actor,
		CharacterID:     e.CharacterID,
		ScopeType:       e.ScopeType,
		ScopeID:         e.ScopeID,
		Announced:       e.Announced,
		AnnouncedPublic: e.AnnouncedPublic,
		Targets:         targets,
		CreatedAt:       e.CreatedAt,
	}
}

func toActivityEntryResponses(entries []models.ActivityEntry) []activityEntryResponse {
	responses := make([]activityEntryResponse, len(entries))
	for i := range entries {
		responses[i] = toActivityEntryResponse(&entries[i])
	}

	return responses
}

// GetCharacterActivityHandler returns one character's activity feed — the
// events visible to that character up to its current game day. The service
// enforces ownership (owner + GM) and strips the actor from announced entries
// for non-GM callers. Must be wrapped in RequireAuth.
func GetCharacterActivityHandler(svc *application.ActivityService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entries, err := svc.Feed(r.Context(), requesterFrom(r), r.PathValue("id"))
		if err != nil {
			writeServiceError(w, err)

			return
		}

		xhttp.WriteJSON(w, http.StatusOK, toActivityEntryResponses(entries))
	})
}

// ListActivityHandler returns the full campaign log, announcement targets
// included. GM only. Must be wrapped in RequireAuth.
func ListActivityHandler(svc *application.ActivityService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entries, err := svc.ListAll(r.Context(), requesterFrom(r))
		if err != nil {
			writeServiceError(w, err)

			return
		}

		xhttp.WriteJSON(w, http.StatusOK, toActivityEntryResponses(entries))
	})
}

type announceActivityRequest struct {
	GameDay      int                   `json:"game_day"`
	Action       models.ActivityAction `json:"action"`
	EntityType   string                `json:"entity_type,omitempty"`
	EntityName   string                `json:"entity_name"`
	Actor        string                `json:"actor,omitempty"`
	Public       bool                  `json:"public,omitempty"`
	CharacterIDs []string              `json:"character_ids,omitempty"`
	GroupIDs     []string              `json:"group_ids,omitempty"`
}

// AnnounceActivityHandler lets a GM broadcast an announced activity entry to
// specific characters, groups, or everyone. Must be wrapped in RequireAuth.
func AnnounceActivityHandler(svc *application.ActivityService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req announceActivityRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			xhttp.WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		entry, err := svc.Announce(r.Context(), requesterFrom(r), &application.AnnounceInput{
			GameDay:      req.GameDay,
			Action:       req.Action,
			EntityType:   req.EntityType,
			EntityName:   req.EntityName,
			Actor:        req.Actor,
			Public:       req.Public,
			CharacterIDs: req.CharacterIDs,
			GroupIDs:     req.GroupIDs,
		})
		if err != nil {
			writeServiceError(w, err)

			return
		}

		xhttp.WriteJSON(w, http.StatusCreated, toActivityEntryResponse(entry))
	})
}
