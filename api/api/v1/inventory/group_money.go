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

// Route paths for a group's money balances, addressed by the group {id}.
const (
	GroupMoneyBasePath     = sessionsv1.GroupPath + "/money"
	GroupMoneyCurrencyPath = GroupMoneyBasePath + "/{currencyId}"
)

// GroupMoneyHandler serves a group's balances under /api/groups/{id}/money.
type GroupMoneyHandler struct {
	inventory *application.InventoryService
}

// NewGroupMoneyHandler builds the group-money handler.
func NewGroupMoneyHandler(inventory *application.InventoryService) *GroupMoneyHandler {
	return &GroupMoneyHandler{inventory: inventory}
}

// Register wires the group-money routes onto r. Each handler must be reached
// through RequireAuth.
func (h *GroupMoneyHandler) Register(r *transport.Router) {
	r.Handle("GET "+GroupMoneyBasePath, h.List())
	r.Handle("PUT "+GroupMoneyCurrencyPath, h.Set())
}

func (h *GroupMoneyHandler) owner(r *http.Request) models.InventoryOwner {
	return models.GroupOwner(r.PathValue("id"))
}

// List returns the group's balances. Callers without access get 404.
func (h *GroupMoneyHandler) List() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		balances, err := h.inventory.ListMoney(r.Context(), transport.RequesterFrom(r), h.owner(r))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		responses := make([]moneyBalanceResponse, len(balances))
		for i := range balances {
			responses[i] = toMoneyBalanceResponse(&balances[i])
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

// Set sets the group's balance in one currency to an absolute amount.
func (h *GroupMoneyHandler) Set() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req setMoneyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		balance, err := h.inventory.SetMoney(
			r.Context(), transport.RequesterFrom(r), h.owner(r), r.PathValue("currencyId"), req.Amount,
		)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toMoneyBalanceResponse(balance))
	})
}
