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

// ListItemDefinitionsHandler returns the item catalog. Must be wrapped in
// RequireAuth.
func ListItemDefinitionsHandler(svc *application.CatalogService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		defs, err := svc.ListItemDefinitions(r.Context())
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

// CreateItemDefinitionHandler lets a GM add an item to the catalog. Must be
// wrapped in RequireAuth.
func CreateItemDefinitionHandler(svc *application.CatalogService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req createItemDefinitionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		d, err := svc.CreateItemDefinition(r.Context(), transport.RequesterFrom(r), req.Name, req.Description, req.Category)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toItemDefinitionResponse(d))
	})
}
