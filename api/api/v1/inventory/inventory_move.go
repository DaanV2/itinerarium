package inventoryv1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
)

// InventoryMovePath transfers item quantity between two inventories.
const InventoryMovePath = "/api/inventory/move"

// InventoryMoveHandler serves item transfers under /api/inventory/move.
type InventoryMoveHandler struct {
	inventory *application.InventoryService
}

// NewInventoryMoveHandler builds the inventory-move handler.
func NewInventoryMoveHandler(inventory *application.InventoryService) *InventoryMoveHandler {
	return &InventoryMoveHandler{inventory: inventory}
}

// Register wires the inventory-move route onto r. The handler must be reached
// through RequireAuth.
func (h *InventoryMoveHandler) Register(r *transport.Router) {
	r.Handle("POST "+InventoryMovePath, h.Move())
}

// Move transfers item quantity between two inventories the caller can access
// (character, group, or location).
func (h *InventoryMoveHandler) Move() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req moveInventoryItemRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		target := models.InventoryOwner{
			CharacterID: req.ToCharacterID,
			GroupID:     req.ToGroupID,
			LocationID:  req.ToLocationID,
		}

		item, err := h.inventory.MoveItem(r.Context(), transport.RequesterFrom(r), req.ItemID, target, req.Quantity)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toInventoryItemResponse(item))
	})
}
