//nolint:dupl // ItemDefinitions and Currencies are parallel catalog repositories with the same CRUD+upsert shape; the one-file-per-entity convention keeps them separate.
package repositories

import (
	"context"
	"errors"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ItemDefinitions provides access to the GM's default item catalog.
type ItemDefinitions struct{ db *persistence.Database }

// NewItemDefinitions builds an ItemDefinitions repository.
func NewItemDefinitions(db *persistence.Database) *ItemDefinitions {
	return &ItemDefinitions{db: db}
}

// Create persists a new item definition.
func (r *ItemDefinitions) Create(ctx context.Context, d *models.ItemDefinition) error {
	err := r.db.DB().WithContext(ctx).Create(d).Error
	if err != nil {
		return err
	}

	return nil
}

// GetByID looks up an item definition by ID, returning ErrNotFound if none
// matches.
func (r *ItemDefinitions) GetByID(ctx context.Context, id string) (*models.ItemDefinition, error) {
	var d models.ItemDefinition

	err := r.db.DB().WithContext(ctx).First(&d, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &d, nil
}

// GetByName looks up an item definition by its unique name, returning
// ErrNotFound if none matches.
func (r *ItemDefinitions) GetByName(ctx context.Context, name string) (*models.ItemDefinition, error) {
	var d models.ItemDefinition

	err := r.db.DB().WithContext(ctx).First(&d, "name = ?", name).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &d, nil
}

// List returns every item definition, ordered by name.
func (r *ItemDefinitions) List(ctx context.Context) ([]models.ItemDefinition, error) {
	var defs []models.ItemDefinition

	err := r.db.DB().WithContext(ctx).Order("name").Find(&defs).Error
	if err != nil {
		return nil, err
	}

	return defs, nil
}

// UpsertByName inserts an item definition or, when one with the same name
// already exists, updates its description and category. Used to seed the
// catalog from a file idempotently across restarts.
func (r *ItemDefinitions) UpsertByName(ctx context.Context, d *models.ItemDefinition) error {
	err := r.db.DB().WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"description", "category", "updated_at"}),
	}).Create(d).Error
	if err != nil {
		return err
	}

	return nil
}
