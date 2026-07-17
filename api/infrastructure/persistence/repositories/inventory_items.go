package repositories

import (
	"context"
	"errors"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
)

// InventoryItems provides access to inventory lines across every owner kind
// (character, group, location).
type InventoryItems struct{ db *persistence.Database }

// NewInventoryItems builds an InventoryItems repository.
func NewInventoryItems(db *persistence.Database) *InventoryItems {
	return &InventoryItems{db: db}
}

// ownerScope narrows a query to one inventory. Rows carry exactly one owner
// column, so matching the set column is sufficient.
func ownerScope(query *gorm.DB, owner models.InventoryOwner) *gorm.DB {
	switch {
	case owner.CharacterID != nil:
		return query.Where("character_id = ?", *owner.CharacterID)
	case owner.GroupID != nil:
		return query.Where("group_id = ?", *owner.GroupID)
	default:
		return query.Where("location_id = ?", *owner.LocationID)
	}
}

// Create persists a new inventory item, recording any given activity entries
// in the same transaction.
func (r *InventoryItems) Create(
	ctx context.Context, item *models.InventoryItem, entries ...*models.ActivityEntry,
) error {
	err := r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(item).Error; err != nil {
			return err
		}

		return createEntries(tx, entries)
	})
	if err != nil {
		return err
	}

	return nil
}

// GetByID looks up an inventory item by ID, returning ErrNotFound if none
// matches.
func (r *InventoryItems) GetByID(ctx context.Context, id string) (*models.InventoryItem, error) {
	var item models.InventoryItem

	err := r.db.DB().WithContext(ctx).First(&item, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &item, nil
}

// ListByOwner returns every line in one inventory, ordered by name.
func (r *InventoryItems) ListByOwner(
	ctx context.Context, owner models.InventoryOwner,
) ([]models.InventoryItem, error) {
	var items []models.InventoryItem

	err := ownerScope(r.db.DB().WithContext(ctx), owner).Order("name").Find(&items).Error
	if err != nil {
		return nil, err
	}

	return items, nil
}

// Update persists changes to an existing inventory item, recording any given
// activity entries in the same transaction.
func (r *InventoryItems) Update(
	ctx context.Context, item *models.InventoryItem, entries ...*models.ActivityEntry,
) error {
	err := r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(item).Error; err != nil {
			return err
		}

		return createEntries(tx, entries)
	})
	if err != nil {
		return err
	}

	return nil
}

// Delete soft-deletes an inventory item, recording any given activity entries
// in the same transaction.
func (r *InventoryItems) Delete(
	ctx context.Context, item *models.InventoryItem, entries ...*models.ActivityEntry,
) error {
	err := r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(item).Error; err != nil {
			return err
		}

		return createEntries(tx, entries)
	})
	if err != nil {
		return err
	}

	return nil
}

// Move transfers quantity units of the source line into the target inventory
// inside one transaction, recording any given activity entries in that same
// transaction. The service layer has already validated access, the quantity
// bound, and that source and target differ; this method owns the mechanics: a
// target line with the same name and catalog reference absorbs the moved
// units, otherwise the line is re-owned (full move) or split (partial move).
// It returns the line now holding the moved units.
func (r *InventoryItems) Move(
	ctx context.Context, source *models.InventoryItem, target models.InventoryOwner, quantity int,
	entries ...*models.ActivityEntry,
) (*models.InventoryItem, error) {
	var result *models.InventoryItem

	err := r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := createEntries(tx, entries); err != nil {
			return err
		}

		match, err := findMatch(tx, target, source)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}

		if match != nil {
			result = match
			match.Quantity += quantity

			if err := tx.Save(match).Error; err != nil {
				return err
			}

			return drainSource(tx, source, quantity)
		}

		if quantity == source.Quantity {
			// Full move with nothing to merge into: re-own the line as-is.
			source.InventoryOwner = target
			result = source

			return tx.Save(source).Error
		}

		// Partial move: split off a new line in the target inventory.
		result = &models.InventoryItem{
			InventoryOwner:   target,
			Name:             source.Name,
			ItemDefinitionID: source.ItemDefinitionID,
			Quantity:         quantity,
			Description:      source.Description,
		}
		if err := tx.Create(result).Error; err != nil {
			return err
		}

		return drainSource(tx, source, quantity)
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// findMatch looks for a line in the target inventory that the moved units can
// merge into: same name and same catalog reference (both free-text or both
// pointing at the same definition). It returns ErrNotFound when the target
// has no such line.
func findMatch(
	tx *gorm.DB, target models.InventoryOwner, source *models.InventoryItem,
) (*models.InventoryItem, error) {
	var match models.InventoryItem

	query := ownerScope(tx, target).Where("name = ?", source.Name)
	// Null-safe catalog match spelled out per case: `IS ?` with a non-NULL
	// parameter is SQLite-only, and the postgres/mysql backends reject it.
	if source.ItemDefinitionID == nil {
		query = query.Where("item_definition_id IS NULL")
	} else {
		query = query.Where("item_definition_id = ?", *source.ItemDefinitionID)
	}

	err := query.First(&match).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &match, nil
}

// drainSource removes quantity units from the source line, deleting the line
// when it hits zero.
func drainSource(tx *gorm.DB, source *models.InventoryItem, quantity int) error {
	if quantity == source.Quantity {
		return tx.Delete(source).Error
	}

	source.Quantity -= quantity

	return tx.Save(source).Error
}
