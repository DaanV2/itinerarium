package repositories

import (
	"context"
	"errors"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
)

// Locations provides access to the campaign's locations.
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

// GetByID looks up a location by ID. It returns ErrNotFound if no location
// matches.
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

// List returns every location, ordered by plane then name.
func (r *Locations) List(ctx context.Context) ([]models.Location, error) {
	var locations []models.Location

	err := r.db.DB().WithContext(ctx).Order("plane").Order("name").Find(&locations).Error
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
