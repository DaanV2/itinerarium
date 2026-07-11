package repositories

import (
	"context"
	"errors"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
)

// Locations provides access to campaign locations (planes, towns, buildings, …).
type Locations struct{ db *persistence.Database }

// NewLocations builds a Locations repository.
func NewLocations(db *persistence.Database) *Locations {
	return &Locations{db: db}
}

// Create persists a new location.
func (r *Locations) Create(ctx context.Context, l *models.Location) error {
	err := r.db.DB().WithContext(ctx).Create(l).Error
	if err != nil {
		return err
	}

	return nil
}

// GetByID looks up a location by ID, returning ErrNotFound if none matches.
func (r *Locations) GetByID(ctx context.Context, id string) (*models.Location, error) {
	var l models.Location

	err := r.db.DB().WithContext(ctx).First(&l, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &l, nil
}

// List returns every location, ordered by name.
func (r *Locations) List(ctx context.Context) ([]models.Location, error) {
	var locations []models.Location

	err := r.db.DB().WithContext(ctx).Order("name").Find(&locations).Error
	if err != nil {
		return nil, err
	}

	return locations, nil
}

// Update persists changes to an existing location.
func (r *Locations) Update(ctx context.Context, l *models.Location) error {
	err := r.db.DB().WithContext(ctx).Save(l).Error
	if err != nil {
		return err
	}

	return nil
}

// Delete soft-deletes a location.
func (r *Locations) Delete(ctx context.Context, l *models.Location) error {
	err := r.db.DB().WithContext(ctx).Delete(l).Error
	if err != nil {
		return err
	}

	return nil
}

// CountChildren reports how many locations name the given location as parent.
// It backs the delete guard: a plane or place with nested locations must not be
// removed out from under them.
func (r *Locations) CountChildren(ctx context.Context, parentID string) (int64, error) {
	var count int64

	err := r.db.DB().WithContext(ctx).
		Model(&models.Location{}).
		Where("parent_id = ?", parentID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}

	return count, nil
}
