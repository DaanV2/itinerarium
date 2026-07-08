package transport

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
)

type addInventoryItemRequest struct {
	Name             string  `json:"name"`
	ItemDefinitionID *string `json:"item_definition_id,omitempty"`
	Quantity         int     `json:"quantity"`
	Description      string  `json:"description,omitempty"`
}

type updateInventoryItemRequest struct {
	Name        *string `json:"name,omitempty"`
	Quantity    *int    `json:"quantity,omitempty"`
	Description *string `json:"description,omitempty"`
}

type inventoryItemResponse struct {
	ID               string  `json:"id"`
	CharacterID      string  `json:"character_id"`
	Name             string  `json:"name"`
	ItemDefinitionID *string `json:"item_definition_id,omitempty"`
	Quantity         int     `json:"quantity"`
	Description      string  `json:"description,omitempty"`
}

func toInventoryItemResponse(item *models.InventoryItem) inventoryItemResponse {
	return inventoryItemResponse{
		ID:               item.ID,
		CharacterID:      item.CharacterID,
		Name:             item.Name,
		ItemDefinitionID: item.ItemDefinitionID,
		Quantity:         item.Quantity,
		Description:      item.Description,
	}
}

type setMoneyRequest struct {
	Amount int64 `json:"amount"`
}

type moneyBalanceResponse struct {
	ID          string `json:"id"`
	CharacterID string `json:"character_id"`
	CurrencyID  string `json:"currency_id"`
	Amount      int64  `json:"amount"`
}

func toMoneyBalanceResponse(b *models.MoneyBalance) moneyBalanceResponse {
	return moneyBalanceResponse{
		ID: b.ID, CharacterID: b.CharacterID, CurrencyID: b.CurrencyID, Amount: b.Amount,
	}
}

// ListInventoryHandler returns a character's inventory. Non-owners get 404.
// Must be wrapped in RequireAuth.
func ListInventoryHandler(svc *application.InventoryService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		items, err := svc.ListInventory(r.Context(), requesterFrom(r), r.PathValue("id"))
		if err != nil {
			writeInventoryServiceError(w, err)

			return
		}

		responses := make([]inventoryItemResponse, len(items))
		for i := range items {
			responses[i] = toInventoryItemResponse(&items[i])
		}

		writeJSON(w, http.StatusOK, responses)
	})
}

// AddInventoryItemHandler appends an item to a character's inventory. Must be
// wrapped in RequireAuth.
func AddInventoryItemHandler(svc *application.InventoryService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req addInventoryItemRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		item, err := svc.AddItem(
			r.Context(), requesterFrom(r), r.PathValue("id"),
			req.Name, req.ItemDefinitionID, req.Quantity, req.Description,
		)
		if err != nil {
			writeInventoryServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, toInventoryItemResponse(item))
	})
}

// UpdateInventoryItemHandler edits one inventory line. Must be wrapped in
// RequireAuth.
func UpdateInventoryItemHandler(svc *application.InventoryService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req updateInventoryItemRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		item, err := svc.UpdateItem(
			r.Context(), requesterFrom(r), r.PathValue("id"), r.PathValue("itemId"),
			req.Name, req.Quantity, req.Description,
		)
		if err != nil {
			writeInventoryServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toInventoryItemResponse(item))
	})
}

// RemoveInventoryItemHandler deletes one inventory line. Must be wrapped in
// RequireAuth.
func RemoveInventoryItemHandler(svc *application.InventoryService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := svc.RemoveItem(r.Context(), requesterFrom(r), r.PathValue("id"), r.PathValue("itemId"))
		if err != nil {
			writeInventoryServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// ListMoneyHandler returns a character's balances. Non-owners get 404. Must be
// wrapped in RequireAuth.
func ListMoneyHandler(svc *application.InventoryService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		balances, err := svc.ListMoney(r.Context(), requesterFrom(r), r.PathValue("id"))
		if err != nil {
			writeInventoryServiceError(w, err)

			return
		}

		responses := make([]moneyBalanceResponse, len(balances))
		for i := range balances {
			responses[i] = toMoneyBalanceResponse(&balances[i])
		}

		writeJSON(w, http.StatusOK, responses)
	})
}

// SetMoneyHandler sets a character's balance in one currency to an absolute
// amount. Must be wrapped in RequireAuth.
func SetMoneyHandler(svc *application.InventoryService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req setMoneyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		balance, err := svc.SetMoney(
			r.Context(), requesterFrom(r), r.PathValue("id"), r.PathValue("currencyId"), req.Amount,
		)
		if err != nil {
			writeInventoryServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toMoneyBalanceResponse(balance))
	})
}

func writeInventoryServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, application.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, application.ErrInvalidName),
		errors.Is(err, application.ErrInvalidQuantity),
		errors.Is(err, application.ErrInvalidAmount),
		errors.Is(err, application.ErrUnknownItemDefinition),
		errors.Is(err, application.ErrUnknownCurrency):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "processing request")
	}
}
