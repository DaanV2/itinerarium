package inventoryv1

import (
	"encoding/json"
	"fmt"
	"net/http"

	charactersv1 "github.com/DaanV2/itinerarium/api/api/v1/characters"
	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
)

// Route paths for a character's inventory, addressed by the character {id}.
const (
	CharacterInventoryBasePath = charactersv1.CharacterPath + "/inventory"
	CharacterInventoryItemPath = CharacterInventoryBasePath + "/{itemId}"
)

// CharacterInventoryHandler serves a character's inventory under
// /api/characters/{id}/inventory.
type CharacterInventoryHandler struct {
	inventory *application.InventoryService
}

// NewCharacterInventoryHandler builds the character-inventory handler.
func NewCharacterInventoryHandler(inventory *application.InventoryService) *CharacterInventoryHandler {
	return &CharacterInventoryHandler{inventory: inventory}
}

// Register wires the character-inventory routes onto r. Each handler must be
// reached through RequireAuth.
func (h *CharacterInventoryHandler) Register(r *transport.Router) {
	r.Handle("GET "+CharacterInventoryBasePath, h.List())
	r.Handle("POST "+CharacterInventoryBasePath, h.Add())
	r.Handle("PATCH "+CharacterInventoryItemPath, h.Update())
	r.Handle("DELETE "+CharacterInventoryItemPath, h.Remove())
}

func (h *CharacterInventoryHandler) owner(r *http.Request) models.InventoryOwner {
	return models.CharacterOwner(r.PathValue("id"))
}

// List returns the character's inventory lines. Callers without access to the
// character get 404.
func (h *CharacterInventoryHandler) List() http.Handler {
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

// Add appends an item to the character's inventory.
func (h *CharacterInventoryHandler) Add() http.Handler {
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

// Update edits one line of the character's inventory.
func (h *CharacterInventoryHandler) Update() http.Handler {
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

// Remove deletes one line from the character's inventory.
func (h *CharacterInventoryHandler) Remove() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		err := h.inventory.RemoveItem(r.Context(), transport.RequesterFrom(r), h.owner(r), r.PathValue("itemId"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
