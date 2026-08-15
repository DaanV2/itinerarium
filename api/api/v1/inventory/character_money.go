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

// Route paths for a character's money balances, addressed by the character {id}.
const (
	CharacterMoneyBasePath     = charactersv1.CharacterPath + "/money"
	CharacterMoneyCurrencyPath = CharacterMoneyBasePath + "/{currencyId}"
)

// CharacterMoneyHandler serves a character's balances under
// /api/characters/{id}/money.
type CharacterMoneyHandler struct {
	inventory *application.InventoryService
}

// NewCharacterMoneyHandler builds the character-money handler.
func NewCharacterMoneyHandler(inventory *application.InventoryService) *CharacterMoneyHandler {
	return &CharacterMoneyHandler{inventory: inventory}
}

// Register wires the character-money routes onto r. Each handler must be
// reached through RequireAuth.
func (h *CharacterMoneyHandler) Register(r *transport.Router) {
	r.Handle("GET "+CharacterMoneyBasePath, h.List())
	r.Handle("PUT "+CharacterMoneyCurrencyPath, h.Set())
}

func (h *CharacterMoneyHandler) owner(r *http.Request) models.InventoryOwner {
	return models.CharacterOwner(r.PathValue("id"))
}

// List returns the character's balances. Callers without access get 404.
func (h *CharacterMoneyHandler) List() http.Handler {
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

// Set sets the character's balance in one currency to an absolute amount.
func (h *CharacterMoneyHandler) Set() http.Handler {
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
