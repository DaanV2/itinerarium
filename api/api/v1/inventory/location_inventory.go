package inventoryv1

import (
	"encoding/json"
	"fmt"
	"net/http"

	locationsv1 "github.com/DaanV2/itinerarium/api/api/v1/locations"
	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
)

// Route paths for a location's inventory, addressed by the location {id}.
const (
	LocationInventoryBasePath = locationsv1.LocationPath + "/inventory"
	LocationInventoryItemPath = LocationInventoryBasePath + "/{itemId}"
)

// LocationInventoryHandler serves a location's inventory under
// /api/locations/{id}/inventory.
type LocationInventoryHandler struct {
	inventory *application.InventoryService
}

// NewLocationInventoryHandler builds the location-inventory handler.
func NewLocationInventoryHandler(inventory *application.InventoryService) *LocationInventoryHandler {
	return &LocationInventoryHandler{inventory: inventory}
}

// Register wires the location-inventory routes onto r. Each handler must be
// reached through RequireAuth.
func (h *LocationInventoryHandler) Register(r *transport.Router) {
	r.Handle("GET "+LocationInventoryBasePath, h.List())
	r.Handle("POST "+LocationInventoryBasePath, h.Add())
	r.Handle("PATCH "+LocationInventoryItemPath, h.Update())
	r.Handle("DELETE "+LocationInventoryItemPath, h.Remove())
}

func (h *LocationInventoryHandler) owner(r *http.Request) models.InventoryOwner {
	return models.LocationOwner(r.PathValue("id"))
}

// List returns the location's inventory lines. Callers without access to the
// location get 404.
func (h *LocationInventoryHandler) List() http.Handler {
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

// Add appends an item to the location's inventory.
func (h *LocationInventoryHandler) Add() http.Handler {
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

// Update edits one line of the location's inventory.
func (h *LocationInventoryHandler) Update() http.Handler {
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

// Remove deletes one line from the location's inventory.
func (h *LocationInventoryHandler) Remove() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		err := h.inventory.RemoveItem(r.Context(), transport.RequesterFrom(r), h.owner(r), r.PathValue("itemId"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
