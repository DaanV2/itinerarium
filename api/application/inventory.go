package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// ErrInvalidQuantity is returned when an inventory quantity is below 1.
var ErrInvalidQuantity = errors.New("invalid quantity")

// ErrInvalidAmount is returned when a money amount is negative.
var ErrInvalidAmount = errors.New("invalid amount")

// ErrUnknownItemDefinition is returned when an inventory item references a
// catalog definition that does not exist.
var ErrUnknownItemDefinition = errors.New("unknown item definition")

// ErrUnknownCurrency is returned when a balance references a currency that is
// not in the catalog.
var ErrUnknownCurrency = errors.New("unknown currency")

// InventoryService manages a character's personal inventory and money. Every
// method first resolves character access through CharacterService.Get, so a
// caller who is neither the owner nor a GM gets ErrNotFound — the inventory's
// existence never leaks (core domain rule 3).
type InventoryService struct {
	characters *CharacterService
	items      *repositories.InventoryItems
	balances   *repositories.MoneyBalances
	currencies *repositories.Currencies
	itemDefs   *repositories.ItemDefinitions
}

// NewInventoryService builds an InventoryService.
func NewInventoryService(
	characters *CharacterService,
	items *repositories.InventoryItems,
	balances *repositories.MoneyBalances,
	currencies *repositories.Currencies,
	itemDefs *repositories.ItemDefinitions,
) *InventoryService {
	return &InventoryService{
		characters: characters,
		items:      items,
		balances:   balances,
		currencies: currencies,
		itemDefs:   itemDefs,
	}
}

// requireCharacterAccess returns ErrNotFound unless the requester owns the
// character or is a GM. It reuses CharacterService.Get so the visibility rule
// lives in exactly one place.
func (s *InventoryService) requireCharacterAccess(ctx context.Context, requester Requester, characterID string) error {
	if _, err := s.characters.Get(ctx, requester, characterID); err != nil {
		return err
	}

	return nil
}

// ListInventory returns the character's inventory lines. Non-owners get
// ErrNotFound.
func (s *InventoryService) ListInventory(
	ctx context.Context, requester Requester, characterID string,
) ([]models.InventoryItem, error) {
	if err := s.requireCharacterAccess(ctx, requester, characterID); err != nil {
		return nil, err
	}

	items, err := s.items.ListByCharacter(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("listing inventory: %w", err)
	}

	return items, nil
}

// AddItem appends an inventory line. Name is required; itemDefinitionID is
// optional and, when set, must reference an existing catalog entry — but
// free-text items (no definition) are always allowed.
func (s *InventoryService) AddItem(
	ctx context.Context, requester Requester, characterID, name string,
	itemDefinitionID *string, quantity int, description string,
) (*models.InventoryItem, error) {
	if err := s.requireCharacterAccess(ctx, requester, characterID); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, ErrInvalidName
	}
	if quantity < 1 {
		return nil, ErrInvalidQuantity
	}
	if err := s.validateItemDefinition(ctx, itemDefinitionID); err != nil {
		return nil, err
	}

	item := &models.InventoryItem{
		CharacterID:      characterID,
		Name:             name,
		ItemDefinitionID: itemDefinitionID,
		Quantity:         quantity,
		Description:      description,
	}
	if err := s.items.Create(ctx, item); err != nil {
		return nil, fmt.Errorf("adding inventory item: %w", err)
	}

	return item, nil
}

// UpdateItem changes an inventory line's name, quantity, and/or description.
func (s *InventoryService) UpdateItem(
	ctx context.Context, requester Requester, characterID, itemID string,
	name *string, quantity *int, description *string,
) (*models.InventoryItem, error) {
	item, err := s.loadOwnedItem(ctx, requester, characterID, itemID)
	if err != nil {
		return nil, err
	}

	if name != nil {
		if *name == "" {
			return nil, ErrInvalidName
		}

		item.Name = *name
	}
	if quantity != nil {
		if *quantity < 1 {
			return nil, ErrInvalidQuantity
		}

		item.Quantity = *quantity
	}
	if description != nil {
		item.Description = *description
	}

	if err := s.items.Update(ctx, item); err != nil {
		return nil, fmt.Errorf("updating inventory item: %w", err)
	}

	return item, nil
}

// RemoveItem deletes an inventory line.
func (s *InventoryService) RemoveItem(
	ctx context.Context, requester Requester, characterID, itemID string,
) error {
	item, err := s.loadOwnedItem(ctx, requester, characterID, itemID)
	if err != nil {
		return err
	}

	if err := s.items.Delete(ctx, item); err != nil {
		return fmt.Errorf("removing inventory item: %w", err)
	}

	return nil
}

// ListMoney returns the character's balances across all currencies.
func (s *InventoryService) ListMoney(
	ctx context.Context, requester Requester, characterID string,
) ([]models.MoneyBalance, error) {
	if err := s.requireCharacterAccess(ctx, requester, characterID); err != nil {
		return nil, err
	}

	balances, err := s.balances.ListByCharacter(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("listing money: %w", err)
	}

	return balances, nil
}

// SetMoney sets the character's balance in one currency to an absolute amount.
func (s *InventoryService) SetMoney(
	ctx context.Context, requester Requester, characterID, currencyID string, amount int64,
) (*models.MoneyBalance, error) {
	if err := s.requireCharacterAccess(ctx, requester, characterID); err != nil {
		return nil, err
	}
	if amount < 0 {
		return nil, ErrInvalidAmount
	}
	if _, err := s.currencies.GetByID(ctx, currencyID); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrUnknownCurrency
		}

		return nil, fmt.Errorf("loading currency: %w", err)
	}

	balance, err := s.balances.Set(ctx, characterID, currencyID, amount)
	if err != nil {
		return nil, fmt.Errorf("setting money: %w", err)
	}

	return balance, nil
}

// loadOwnedItem fetches an inventory line and confirms it belongs to the
// character the requester may access. A line belonging to a different
// character is reported as ErrNotFound, never surfaced.
func (s *InventoryService) loadOwnedItem(
	ctx context.Context, requester Requester, characterID, itemID string,
) (*models.InventoryItem, error) {
	if err := s.requireCharacterAccess(ctx, requester, characterID); err != nil {
		return nil, err
	}

	item, err := s.items.GetByID(ctx, itemID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("loading inventory item: %w", err)
	}
	if item.CharacterID != characterID {
		return nil, ErrNotFound
	}

	return item, nil
}

// validateItemDefinition confirms an optional catalog reference exists.
func (s *InventoryService) validateItemDefinition(ctx context.Context, itemDefinitionID *string) error {
	if itemDefinitionID == nil {
		return nil
	}

	if _, err := s.itemDefs.GetByID(ctx, *itemDefinitionID); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return ErrUnknownItemDefinition
		}

		return fmt.Errorf("loading item definition: %w", err)
	}

	return nil
}
