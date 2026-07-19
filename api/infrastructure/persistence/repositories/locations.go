package repositories

import (
	"context"
	"errors"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Locations provides access to campaign locations and their description
// sections.
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

// GetByID looks up a location by ID with its description sections preloaded
// in order, returning ErrNotFound if none matches.
func (r *Locations) GetByID(ctx context.Context, id string) (*models.Location, error) {
	var l models.Location

	err := r.db.DB().WithContext(ctx).
		Preload("Sections", func(db *gorm.DB) *gorm.DB { return db.Order("position") }).
		First(&l, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &l, nil
}

// List returns every location, ordered by plane then name (GM-wide view).
// Description sections are not preloaded — callers needing them use GetByID.
func (r *Locations) List(ctx context.Context) ([]models.Location, error) {
	var locations []models.Location

	err := r.db.DB().WithContext(ctx).Order("plane, name").Find(&locations).Error
	if err != nil {
		return nil, err
	}

	return locations, nil
}

// ListByIDs returns the locations with the given IDs, ordered by plane then
// name. An empty ID list returns no rows.
func (r *Locations) ListByIDs(ctx context.Context, ids []string) ([]models.Location, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var locations []models.Location

	err := r.db.DB().WithContext(ctx).Where("id IN ?", ids).Order("plane, name").Find(&locations).Error
	if err != nil {
		return nil, err
	}

	return locations, nil
}

// Update persists the location's own fields and replaces its description
// section rows with the given final list, all in one transaction. Sections
// carrying an ID are updated in place (so player edits keep GM-only rows
// untouched), sections without an ID are inserted, and existing rows absent
// from the list are deleted.
func (r *Locations) Update(ctx context.Context, l *models.Location, sections []models.LocationSection) error {
	return r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit(clause.Associations).Save(l).Error; err != nil {
			return err
		}

		keep := make([]string, 0, len(sections))
		for i := range sections {
			sections[i].LocationID = l.ID
			sections[i].Position = i
			if err := tx.Omit(clause.Associations).Save(&sections[i]).Error; err != nil {
				return err
			}

			keep = append(keep, sections[i].ID)
		}

		query := tx.Where("location_id = ?", l.ID)
		if len(keep) > 0 {
			query = query.Where("id NOT IN ?", keep)
		}

		return query.Delete(&models.LocationSection{}).Error
	})
}
