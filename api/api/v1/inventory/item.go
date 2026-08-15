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

type createItemDefinitionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
}

type itemDefinitionResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
}

func toItemDefinitionResponse(d *models.ItemDefinition) itemDefinitionResponse {
	return itemDefinitionResponse{ID: d.ID, Name: d.Name, Description: d.Description, Category: d.Category}
}

// ItemsPath is the item-definition catalog.
const ItemsPath = "/api/items"

// ItemHandler serves the item-definition catalog under /api/items.
type ItemHandler struct {
	catalog *application.CatalogService
}

// NewItemHandler builds the item-catalog handler.
func NewItemHandler(catalog *application.CatalogService) *ItemHandler {
	return &ItemHandler{catalog: catalog}
}

// Register wires the item-catalog routes onto r. Each handler must be reached
// through RequireAuth.
func (h *ItemHandler) Register(r *transport.Router) {
	r.Handle("GET "+ItemsPath, h.List())
	r.Handle("POST "+ItemsPath, h.Create())
}

// List returns the item catalog.
func (h *ItemHandler) List() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		defs, err := h.catalog.ListItemDefinitions(r.Context())
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		responses := make([]itemDefinitionResponse, len(defs))
		for i := range defs {
			responses[i] = toItemDefinitionResponse(&defs[i])
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

// Create lets a GM add an item to the catalog.
func (h *ItemHandler) Create() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req createItemDefinitionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		d, err := h.catalog.CreateItemDefinition(
			r.Context(), transport.RequesterFrom(r), req.Name, req.Description, req.Category,
		)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toItemDefinitionResponse(d))
	})
}
