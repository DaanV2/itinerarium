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

// Route paths for the group resource. GroupsPath / GroupPath are the shared
// bases that group subresource groups in other packages (inventory, money)
// build their own paths from.
const (
	GroupsPath       = "/api/groups"
	GroupPath        = GroupsPath + "/{id}"
	GroupMembersPath = GroupPath + "/members"
	GroupMemberPath  = GroupMembersPath + "/{characterId}"
)

// GroupHandler serves groups and membership under /api/groups.
type GroupHandler struct {
	groups *application.GroupService
}

// NewGroupHandler builds the group resource handler.
func NewGroupHandler(groups *application.GroupService) *GroupHandler {
	return &GroupHandler{groups: groups}
}

// Register wires the group routes onto r. Each handler must be reached through
// RequireAuth.
func (h *GroupHandler) Register(r *transport.Router) {
	r.Handle("GET "+GroupsPath, h.List())
	r.Handle("POST "+GroupsPath, h.Create())
	r.Handle("GET "+GroupPath, h.Get())
	r.Handle("PATCH "+GroupPath, h.Update())
	r.Handle("POST "+GroupMembersPath, h.Join())
	r.Handle("DELETE "+GroupMemberPath, h.Leave())
}

// Create lets a GM create a group.
func (h *GroupHandler) Create() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req createGroupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		group, err := h.groups.Create(r.Context(), transport.RequesterFrom(r), req.Name, req.Type, req.Description)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toGroupResponse(group))
	})
}

// List returns every group with its members.
func (h *GroupHandler) List() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		groups, err := h.groups.List(r.Context(), transport.RequesterFrom(r))
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

// Get returns one group with its members.
func (h *GroupHandler) Get() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		group, err := h.groups.Get(r.Context(), transport.RequesterFrom(r), r.PathValue("id"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toGroupResponse(group))
	})
}

// Update lets a GM edit a group's name, type, or description.
func (h *GroupHandler) Update() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req updateGroupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		group, err := h.groups.Update(
			r.Context(), transport.RequesterFrom(r), r.PathValue("id"), req.Name, req.Type, req.Description,
		)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toGroupResponse(group))
	})
}

// Join adds one of the caller's characters (or any character, for a GM) to a
// group.
func (h *GroupHandler) Join() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req joinGroupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		if err := h.groups.Join(r.Context(), transport.RequesterFrom(r), r.PathValue("id"), req.CharacterID); err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// Leave removes one of the caller's characters (or any character, for a GM)
// from a group.
func (h *GroupHandler) Leave() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		err := h.groups.Leave(r.Context(), transport.RequesterFrom(r), r.PathValue("id"), r.PathValue("characterId"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
