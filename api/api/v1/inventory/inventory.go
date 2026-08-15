// Package inventoryv1 serves inventory, money, item-catalog, and item-move
// endpoints. Inventory and money are owned by a character, group, or location;
// each owner kind has its own handler group (CharacterInventoryHandler,
// GroupInventoryHandler, LocationInventoryHandler, and the money equivalents)
// so the route, not a shared parameter, decides the owner. The request/response
// DTOs and mappers below are shared by those groups.
package inventoryv1

import (
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
