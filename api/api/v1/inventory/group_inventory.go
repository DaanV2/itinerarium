package inventoryv1

import (
	"encoding/json"
	"fmt"
	"net/http"

	sessionsv1 "github.com/DaanV2/itinerarium/api/api/v1/sessions"
	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
)

// Route paths for a group's inventory, addressed by the group {id}.
const (
	GroupInventoryBasePath = sessionsv1.GroupPath + "/inventory"
	GroupInventoryItemPath = GroupInventoryBasePath + "/{itemId}"
)

// GroupInventoryHandler serves a group's inventory under
// /api/groups/{id}/inventory.
type GroupInventoryHandler struct {
	inventory *application.InventoryService
}

// NewGroupInventoryHandler builds the group-inventory handler.
func NewGroupInventoryHandler(inventory *application.InventoryService) *GroupInventoryHandler {
	return &GroupInventoryHandler{inventory: inventory}
}

// Register wires the group-inventory routes onto r. Each handler must be
// reached through RequireAuth.
func (h *GroupInventoryHandler) Register(r *transport.Router) {
	r.Handle("GET "+GroupInventoryBasePath, h.List())
	r.Handle("POST "+GroupInventoryBasePath, h.Add())
	r.Handle("PATCH "+GroupInventoryItemPath, h.Update())
	r.Handle("DELETE "+GroupInventoryItemPath, h.Remove())
}

func (h *GroupInventoryHandler) owner(r *http.Request) models.InventoryOwner {
	return models.GroupOwner(r.PathValue("id"))
}

// List returns the group's inventory lines. Callers without access to the
// group get 404.
func (h *GroupInventoryHandler) List() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		items, err := h.inventory.ListInventory(r.Context(), transport.RequesterFrom(r), h.owner(r))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		responses := make([]inventoryItemResponse, len(items))
		for i := range items {
			responses[i] = toInventoryItemResponse(&items[i])
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

// Add appends an item to the group's inventory.
func (h *GroupInventoryHandler) Add() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req addInventoryItemRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		item, err := h.inventory.AddItem(
			r.Context(), transport.RequesterFrom(r), h.owner(r),
			req.Name, req.ItemDefinitionID, req.Quantity, req.Description,
		)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toInventoryItemResponse(item))
	})
}

// Update edits one line of the group's inventory.
func (h *GroupInventoryHandler) Update() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req updateInventoryItemRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		item, err := h.inventory.UpdateItem(
			r.Context(), transport.RequesterFrom(r), h.owner(r), r.PathValue("itemId"),
			req.Name, req.Quantity, req.Description,
		)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toInventoryItemResponse(item))
	})
}

// Remove deletes one line from the group's inventory.
func (h *GroupInventoryHandler) Remove() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		err := h.inventory.RemoveItem(r.Context(), transport.RequesterFrom(r), h.owner(r), r.PathValue("itemId"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
