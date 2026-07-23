package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
	"github.com/DaanV2/itinerarium/api/transport"
)

// OwnerExtractor derives the addressed inventory owner from the request path.
// The same handlers serve character, group, and location inventories; the
// route decides which owner kind the {id} parameter names.
type OwnerExtractor func(r *http.Request) models.InventoryOwner

// CharacterOwner reads {id} as a character inventory address.
func CharacterOwner(r *http.Request) models.InventoryOwner {
	return models.CharacterOwner(r.PathValue("id"))
}

// GroupOwner reads {id} as a group inventory address.
func GroupOwner(r *http.Request) models.InventoryOwner {
	return models.GroupOwner(r.PathValue("id"))
}

// LocationOwner reads {id} as a location inventory address.
func LocationOwner(r *http.Request) models.InventoryOwner {
	return models.LocationOwner(r.PathValue("id"))
}

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
	CharacterID      *string `json:"character_id,omitempty"`
	GroupID          *string `json:"group_id,omitempty"`
	LocationID       *string `json:"location_id,omitempty"`
	Name             string  `json:"name"`
	ItemDefinitionID *string `json:"item_definition_id,omitempty"`
	Quantity         int     `json:"quantity"`
	Description      string  `json:"description,omitempty"`
}

func toInventoryItemResponse(item *models.InventoryItem) inventoryItemResponse {
	return inventoryItemResponse{
		ID:               item.ID,
		CharacterID:      item.CharacterID,
		GroupID:          item.GroupID,
		LocationID:       item.LocationID,
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
	ID          string  `json:"id"`
	CharacterID *string `json:"character_id,omitempty"`
	GroupID     *string `json:"group_id,omitempty"`
	CurrencyID  string  `json:"currency_id"`
	Amount      int64   `json:"amount"`
}

func toMoneyBalanceResponse(b *models.MoneyBalance) moneyBalanceResponse {
	return moneyBalanceResponse{
		ID: b.ID, CharacterID: b.CharacterID, GroupID: b.GroupID, CurrencyID: b.CurrencyID, Amount: b.Amount,
	}
}

type moveInventoryItemRequest struct {
	ItemID        string  `json:"item_id"`
	ToCharacterID *string `json:"to_character_id,omitempty"`
	ToGroupID     *string `json:"to_group_id,omitempty"`
	ToLocationID  *string `json:"to_location_id,omitempty"`
	Quantity      int     `json:"quantity"`
}

// ListInventoryHandler returns an inventory's lines. Callers without access
// to the owner get 404. Must be wrapped in RequireAuth.
func ListInventoryHandler(svc *application.InventoryService, owner OwnerExtractor) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		items, err := svc.ListInventory(r.Context(), transport.RequesterFrom(r), owner(r))
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

// AddInventoryItemHandler appends an item to an inventory. Must be wrapped in
// RequireAuth.
func AddInventoryItemHandler(svc *application.InventoryService, owner OwnerExtractor) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req addInventoryItemRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		item, err := svc.AddItem(
			r.Context(), transport.RequesterFrom(r), owner(r),
			req.Name, req.ItemDefinitionID, req.Quantity, req.Description,
		)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toInventoryItemResponse(item))
	})
}

// UpdateInventoryItemHandler edits one inventory line. Must be wrapped in
// RequireAuth.
func UpdateInventoryItemHandler(svc *application.InventoryService, owner OwnerExtractor) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req updateInventoryItemRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		item, err := svc.UpdateItem(
			r.Context(), transport.RequesterFrom(r), owner(r), r.PathValue("itemId"),
			req.Name, req.Quantity, req.Description,
		)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toInventoryItemResponse(item))
	})
}

// RemoveInventoryItemHandler deletes one inventory line. Must be wrapped in
// RequireAuth.
func RemoveInventoryItemHandler(svc *application.InventoryService, owner OwnerExtractor) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		err := svc.RemoveItem(r.Context(), transport.RequesterFrom(r), owner(r), r.PathValue("itemId"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// MoveInventoryItemHandler transfers item quantity between two inventories
// the caller can access (character, group, or location). Must be wrapped in
// RequireAuth.
func MoveInventoryItemHandler(svc *application.InventoryService) http.Handler {
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

		item, err := svc.MoveItem(r.Context(), transport.RequesterFrom(r), req.ItemID, target, req.Quantity)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toInventoryItemResponse(item))
	})
}

// ListMoneyHandler returns an owner's balances. Callers without access get
// 404. Must be wrapped in RequireAuth.
func ListMoneyHandler(svc *application.InventoryService, owner OwnerExtractor) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		balances, err := svc.ListMoney(r.Context(), transport.RequesterFrom(r), owner(r))
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

// SetMoneyHandler sets an owner's balance in one currency to an absolute
// amount. Must be wrapped in RequireAuth.
func SetMoneyHandler(svc *application.InventoryService, owner OwnerExtractor) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req setMoneyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		balance, err := svc.SetMoney(
			r.Context(), transport.RequesterFrom(r), owner(r), r.PathValue("currencyId"), req.Amount,
		)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toMoneyBalanceResponse(balance))
	})
}
