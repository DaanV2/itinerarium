package activitiesv1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	charactersv1 "github.com/DaanV2/itinerarium/api/api/v1/characters"
	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
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

// CharacterActivityPath is one character's activity feed, addressed by the
// character {id}.
const CharacterActivityPath = charactersv1.CharacterPath + "/activity"

// CharacterActivityHandler serves one character's activity feed under
// /api/characters/{id}/activity.
type CharacterActivityHandler struct {
	activity *application.ActivityService
}

// NewCharacterActivityHandler builds the character-activity handler.
func NewCharacterActivityHandler(activity *application.ActivityService) *CharacterActivityHandler {
	return &CharacterActivityHandler{activity: activity}
}

// Register wires the character-activity route onto r. The handler must be
// reached through RequireAuth.
func (h *CharacterActivityHandler) Register(r *transport.Router) {
	r.Handle("GET "+CharacterActivityPath, h.Feed())
}

// Feed returns one character's activity feed — the events visible to that
// character up to its current game day. The service enforces ownership (owner +
// GM) and strips the actor from announced entries for non-GM callers.
func (h *CharacterActivityHandler) Feed() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		entries, err := h.activity.Feed(r.Context(), transport.RequesterFrom(r), r.PathValue("id"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toActivityEntryResponses(entries))
	})
}

// Route paths for the campaign-wide activity log.
const (
	ActivityPath             = "/api/activity"
	ActivityAnnouncementPath = ActivityPath + "/announcements"
)

// ActivityHandler serves the GM-wide campaign log and announcements under
// /api/activity. The per-character feed lives under
// /api/characters/{id}/activity.
type ActivityHandler struct {
	activity *application.ActivityService
}

// NewActivityHandler builds the campaign-log handler.
func NewActivityHandler(activity *application.ActivityService) *ActivityHandler {
	return &ActivityHandler{activity: activity}
}

// Register wires the campaign-log routes onto r. Each handler must be reached
// through RequireAuth.
func (h *ActivityHandler) Register(r *transport.Router) {
	r.Handle("GET "+ActivityPath, h.List())
	r.Handle("POST "+ActivityAnnouncementPath, h.Announce())
}

// List returns the full campaign log, announcement targets included. GM only.
func (h *ActivityHandler) List() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		entries, err := h.activity.ListAll(r.Context(), transport.RequesterFrom(r))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toActivityEntryResponses(entries))
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

// Announce lets a GM broadcast an announced activity entry to specific
// characters, groups, or everyone.
func (h *ActivityHandler) Announce() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req announceActivityRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		entry, err := h.activity.Announce(r.Context(), transport.RequesterFrom(r), &application.AnnounceInput{
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
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toActivityEntryResponse(entry))
	})
}
