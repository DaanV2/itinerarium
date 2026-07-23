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

type createGroupRequest struct {
	Name        string           `json:"name"`
	Type        models.GroupType `json:"type"`
	Description string           `json:"description,omitempty"`
}

type updateGroupRequest struct {
	Name        *string           `json:"name,omitempty"`
	Type        *models.GroupType `json:"type,omitempty"`
	Description *string           `json:"description,omitempty"`
}

type joinGroupRequest struct {
	CharacterID string `json:"character_id"`
}

// groupMemberResponse deliberately exposes only a member's identity — not the
// character's game day or owner, which are nobody else's business.
type groupMemberResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type groupResponse struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Type        models.GroupType      `json:"type"`
	Description string                `json:"description,omitempty"`
	Members     []groupMemberResponse `json:"members"`
}

func toGroupResponse(g *models.Group) groupResponse {
	members := make([]groupMemberResponse, len(g.Members))
	for i := range g.Members {
		members[i] = groupMemberResponse{ID: g.Members[i].ID, Name: g.Members[i].Name}
	}

	return groupResponse{
		ID: g.ID, Name: g.Name, Type: g.Type, Description: g.Description, Members: members,
	}
}

// CreateGroupHandler lets a GM create a group. Must be wrapped in RequireAuth.
func CreateGroupHandler(svc *application.GroupService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req createGroupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		group, err := svc.Create(r.Context(), transport.RequesterFrom(r), req.Name, req.Type, req.Description)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toGroupResponse(group))
	})
}

// ListGroupsHandler returns every group with its members. Must be wrapped in
// RequireAuth.
func ListGroupsHandler(svc *application.GroupService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		groups, err := svc.List(r.Context(), transport.RequesterFrom(r))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		responses := make([]groupResponse, len(groups))
		for i := range groups {
			responses[i] = toGroupResponse(&groups[i])
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

// GetGroupHandler returns one group with its members. Must be wrapped in
// RequireAuth.
func GetGroupHandler(svc *application.GroupService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		group, err := svc.Get(r.Context(), transport.RequesterFrom(r), r.PathValue("id"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toGroupResponse(group))
	})
}

// UpdateGroupHandler lets a GM edit a group's name, type, or description.
// Must be wrapped in RequireAuth.
func UpdateGroupHandler(svc *application.GroupService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req updateGroupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		group, err := svc.Update(r.Context(), transport.RequesterFrom(r), r.PathValue("id"), req.Name, req.Type, req.Description)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toGroupResponse(group))
	})
}

// JoinGroupHandler adds one of the caller's characters (or any character, for
// a GM) to a group. Must be wrapped in RequireAuth.
func JoinGroupHandler(svc *application.GroupService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req joinGroupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		if err := svc.Join(r.Context(), transport.RequesterFrom(r), r.PathValue("id"), req.CharacterID); err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// LeaveGroupHandler removes one of the caller's characters (or any character,
// for a GM) from a group. Must be wrapped in RequireAuth.
func LeaveGroupHandler(svc *application.GroupService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		err := svc.Leave(r.Context(), transport.RequesterFrom(r), r.PathValue("id"), r.PathValue("characterId"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
