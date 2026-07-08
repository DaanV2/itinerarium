package repositories

import (
	"context"
	"errors"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
)

// InventoryItems provides access to character inventory lines.
type InventoryItems struct{ db *persistence.Database }

// NewInventoryItems builds an InventoryItems repository.
func NewInventoryItems(db *persistence.Database) *InventoryItems {
	return &InventoryItems{db: db}
}

// Create persists a new inventory item.
func (r *InventoryItems) Create(ctx context.Context, item *models.InventoryItem) error {
	err := r.db.DB().WithContext(ctx).Create(item).Error
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

// ListByCharacter returns every inventory line owned by the character, ordered
// by name.
func (r *InventoryItems) ListByCharacter(ctx context.Context, characterID string) ([]models.InventoryItem, error) {
	var items []models.InventoryItem

	err := r.db.DB().WithContext(ctx).
		Where("character_id = ?", characterID).
		Order("name").
		Find(&items).Error
	if err != nil {
		return nil, err
	}

	return items, nil
}

// Update persists changes to an existing inventory item.
func (r *InventoryItems) Update(ctx context.Context, item *models.InventoryItem) error {
	err := r.db.DB().WithContext(ctx).Save(item).Error
	if err != nil {
		return err
	}

	return nil
}

// Delete soft-deletes an inventory item.
func (r *InventoryItems) Delete(ctx context.Context, item *models.InventoryItem) error {
	err := r.db.DB().WithContext(ctx).Delete(item).Error
	if err != nil {
		return err
	}

	return nil
}
