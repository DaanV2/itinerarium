package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/google/uuid"
)

// ErrInvalidQuantity is returned when an inventory quantity is below 1, or a
// move asks for more units than the source line holds.
var ErrInvalidQuantity = serviceErr(KindValidation, "invalid quantity")

// ErrInvalidAmount is returned when a money amount is negative.
var ErrInvalidAmount = serviceErr(KindValidation, "invalid amount")

// ErrUnknownItemDefinition is returned when an inventory item references a
// catalog definition that does not exist.
var ErrUnknownItemDefinition = serviceErr(KindValidation, "unknown item definition")

// ErrUnknownCurrency is returned when a balance references a currency that is
// not in the catalog.
var ErrUnknownCurrency = serviceErr(KindValidation, "unknown currency")

// ErrInvalidOwner is returned when an inventory owner does not identify
// exactly one character, group, or location — or names a kind the operation
// does not support (locations hold items, not money).
var ErrInvalidOwner = serviceErr(KindValidation, "invalid inventory owner")

// ErrSameInventory is returned when a move targets the inventory the item is
// already in.
var ErrSameInventory = serviceErr(KindValidation, "cannot move within the same inventory")

// InventoryService manages the inventories and money of characters, groups,
// and locations, plus item movement between them. Every method resolves
// access to the owning entity first, and every failure reads as ErrNotFound —
// an inventory the requester may not see does not exist as far as they can
// tell (core domain rule 3). Access is a single level: whoever can view an
// inventory can modify it.
//
//   - character inventories/money: the owning player and GMs
//   - group inventories/money: owners of a current member character and GMs
//   - location inventories: characters holding a LocationAccess grant
//     (directly or via a group) and GMs
type InventoryService struct {
	characters    *CharacterService
	locations     *LocationService
	groups        *repositories.Groups
	characterRepo *repositories.Characters
	items         *repositories.InventoryItems
	balances      *repositories.MoneyBalances
	currencies    *repositories.Currencies
	itemDefs      *repositories.ItemDefinitions
}

// NewInventoryService builds an InventoryService.
func NewInventoryService(
	characters *CharacterService,
	locations *LocationService,
	groups *repositories.Groups,
	characterRepo *repositories.Characters,
	items *repositories.InventoryItems,
	balances *repositories.MoneyBalances,
	currencies *repositories.Currencies,
	itemDefs *repositories.ItemDefinitions,
) *InventoryService {
	return &InventoryService{
		characters:    characters,
		locations:     locations,
		groups:        groups,
		characterRepo: characterRepo,
		items:         items,
		balances:      balances,
		currencies:    currencies,
		itemDefs:      itemDefs,
	}
}

// requireOwnerAccess returns ErrNotFound unless the requester may use the
// owner's inventory. Each owner kind delegates to the service that already
// owns its visibility rule, so every rule lives in exactly one place.
func (s *InventoryService) requireOwnerAccess(
	ctx context.Context, requester Requester, owner models.InventoryOwner,
) error {
	if !owner.Valid() {
		return ErrInvalidOwner
	}

	switch {
	case owner.CharacterID != nil:
		_, err := s.characters.Get(ctx, requester, *owner.CharacterID)

		return err
	case owner.GroupID != nil:
		return s.requireGroupAccess(ctx, requester, *owner.GroupID)
	default:
		_, err := s.locations.Get(ctx, requester, *owner.LocationID)

		return err
	}
}

// requireGroupAccess returns ErrNotFound unless the requester is a GM or owns
// a character that is currently a member of the group. A group's existence is
// public, but its shared inventory and money are member-only content.
func (s *InventoryService) requireGroupAccess(ctx context.Context, requester Requester, groupID string) error {
	if _, err := s.groups.GetByID(ctx, groupID); err != nil {
		return notFoundOr(err, "loading group")
	}
	if requester.IsGM() {
		return nil
	}

	characters, err := s.characterRepo.ListByUser(ctx, requester.UserID())
	if err != nil {
		return fmt.Errorf("listing requester characters: %w", err)
	}

	characterIDs := make([]string, len(characters))
	for i := range characters {
		characterIDs[i] = characters[i].ID
	}

	memberOf, err := s.groups.GroupIDsForCharacters(ctx, characterIDs)
	if err != nil {
		return fmt.Errorf("resolving requester groups: %w", err)
	}
	if !slices.Contains(memberOf, groupID) {
		return ErrNotFound
	}

	return nil
}

// ListInventory returns an inventory's lines. Callers without access get
// ErrNotFound.
func (s *InventoryService) ListInventory(
	ctx context.Context, requester Requester, owner models.InventoryOwner,
) ([]models.InventoryItem, error) {
	if err := s.requireOwnerAccess(ctx, requester, owner); err != nil {
		return nil, err
	}

	items, err := s.items.ListByOwner(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("listing inventory: %w", err)
	}

	return items, nil
}

// AddItem appends an inventory line. Name is required; itemDefinitionID is
// optional and, when set, must reference an existing catalog entry — but
// free-text items (no definition) are always allowed.
func (s *InventoryService) AddItem(
	ctx context.Context, requester Requester, owner models.InventoryOwner, name string,
	itemDefinitionID *string, quantity int, description string,
) (*models.InventoryItem, error) {
	if err := s.requireOwnerAccess(ctx, requester, owner); err != nil {
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
		InventoryOwner:   owner,
		Name:             name,
		ItemDefinitionID: itemDefinitionID,
		Quantity:         quantity,
		Description:      description,
	}
	// Pre-assign the ID so the activity entry can reference the new line.
	item.ID = uuid.NewString()

	entries, err := s.inventoryEntry(ctx, requester, owner, models.ActivityActionAdded, "item", item.ID, item.Name)
	if err != nil {
		return nil, err
	}
	if err := s.items.Create(ctx, item, entries...); err != nil {
		return nil, fmt.Errorf("adding inventory item: %w", err)
	}

	return item, nil
}

// UpdateItem changes an inventory line's name, quantity, and/or description.
func (s *InventoryService) UpdateItem(
	ctx context.Context, requester Requester, owner models.InventoryOwner, itemID string,
	name *string, quantity *int, description *string,
) (*models.InventoryItem, error) {
	item, err := s.loadOwnedItem(ctx, requester, owner, itemID)
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

	entries, err := s.inventoryEntry(ctx, requester, owner, models.ActivityActionUpdated, "item", item.ID, item.Name)
	if err != nil {
		return nil, err
	}
	if err := s.items.Update(ctx, item, entries...); err != nil {
		return nil, fmt.Errorf("updating inventory item: %w", err)
	}

	return item, nil
}

// RemoveItem deletes an inventory line.
func (s *InventoryService) RemoveItem(
	ctx context.Context, requester Requester, owner models.InventoryOwner, itemID string,
) error {
	item, err := s.loadOwnedItem(ctx, requester, owner, itemID)
	if err != nil {
		return err
	}

	entries, err := s.inventoryEntry(ctx, requester, owner, models.ActivityActionRemoved, "item", item.ID, item.Name)
	if err != nil {
		return err
	}
	if err := s.items.Delete(ctx, item, entries...); err != nil {
		return fmt.Errorf("removing inventory item: %w", err)
	}

	return nil
}

// MoveItem transfers quantity units of an item into another inventory. The
// requester needs access to both ends: without source access the item itself
// reads as ErrNotFound, without target access the target does. Moving less
// than the full line splits it; a matching line in the target (same name and
// catalog reference) absorbs the units instead of duplicating the row.
func (s *InventoryService) MoveItem(
	ctx context.Context, requester Requester, itemID string, target models.InventoryOwner, quantity int,
) (*models.InventoryItem, error) {
	if !target.Valid() {
		return nil, ErrInvalidOwner
	}

	item, err := s.items.GetByID(ctx, itemID)
	if err != nil {
		return nil, notFoundOr(err, "loading inventory item")
	}

	// Source access first: a caller who cannot see the source inventory must
	// not learn the item exists, whatever else is wrong with the request.
	if err := s.requireOwnerAccess(ctx, requester, item.InventoryOwner); err != nil {
		return nil, err
	}
	if target.Equal(item.InventoryOwner) {
		return nil, ErrSameInventory
	}
	if err := s.requireOwnerAccess(ctx, requester, target); err != nil {
		return nil, err
	}
	if quantity < 1 || quantity > item.Quantity {
		return nil, ErrInvalidQuantity
	}

	// A move is a removal from the source scope and an addition to the target
	// scope; either half is skipped when that end is a private character
	// inventory. The added entry carries no entity ID — the receiving line may
	// be a merge into an existing row or a fresh split, decided inside Move.
	entries, err := s.inventoryEntry(
		ctx, requester, item.InventoryOwner, models.ActivityActionRemoved, "item", item.ID, item.Name,
	)
	if err != nil {
		return nil, err
	}
	addedEntries, err := s.inventoryEntry(ctx, requester, target, models.ActivityActionAdded, "item", "", item.Name)
	if err != nil {
		return nil, err
	}
	entries = append(entries, addedEntries...)

	moved, err := s.items.Move(ctx, item, target, quantity, entries...)
	if err != nil {
		return nil, fmt.Errorf("moving inventory item: %w", err)
	}

	return moved, nil
}

// ListMoney returns the owner's balances across all currencies. Only
// characters and groups hold money.
func (s *InventoryService) ListMoney(
	ctx context.Context, requester Requester, owner models.InventoryOwner,
) ([]models.MoneyBalance, error) {
	if owner.LocationID != nil {
		return nil, ErrInvalidOwner
	}
	if err := s.requireOwnerAccess(ctx, requester, owner); err != nil {
		return nil, err
	}

	balances, err := s.balances.ListByOwner(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("listing money: %w", err)
	}

	return balances, nil
}

// SetMoney sets the owner's balance in one currency to an absolute amount.
func (s *InventoryService) SetMoney(
	ctx context.Context, requester Requester, owner models.InventoryOwner, currencyID string, amount int64,
) (*models.MoneyBalance, error) {
	if owner.LocationID != nil {
		return nil, ErrInvalidOwner
	}
	if err := s.requireOwnerAccess(ctx, requester, owner); err != nil {
		return nil, err
	}
	if amount < 0 {
		return nil, ErrInvalidAmount
	}
	currency, err := s.currencies.GetByID(ctx, currencyID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrUnknownCurrency
		}

		return nil, fmt.Errorf("loading currency: %w", err)
	}

	entries, err := s.inventoryEntry(ctx, requester, owner, models.ActivityActionUpdated, "money", currency.ID, currency.Name)
	if err != nil {
		return nil, err
	}

	balance, err := s.balances.Set(ctx, owner, currencyID, amount, entries...)
	if err != nil {
		return nil, fmt.Errorf("setting money: %w", err)
	}

	return balance, nil
}

// loadOwnedItem fetches an inventory line and confirms it belongs to the
// inventory the requester addressed. A line living elsewhere is reported as
// ErrNotFound, never surfaced.
func (s *InventoryService) loadOwnedItem(
	ctx context.Context, requester Requester, owner models.InventoryOwner, itemID string,
) (*models.InventoryItem, error) {
	if err := s.requireOwnerAccess(ctx, requester, owner); err != nil {
		return nil, err
	}

	item, err := s.items.GetByID(ctx, itemID)
	if err != nil {
		return nil, notFoundOr(err, "loading inventory item")
	}
	if !owner.Equal(item.InventoryOwner) {
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
