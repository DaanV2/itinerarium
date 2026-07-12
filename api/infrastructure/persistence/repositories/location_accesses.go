package repositories

import (
	"context"
	"errors"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
)

// LocationAccesses provides access to per-character and per-group location
// grants.
type LocationAccesses struct{ db *persistence.Database }

// NewLocationAccesses builds a LocationAccesses repository.
func NewLocationAccesses(db *persistence.Database) *LocationAccesses {
	return &LocationAccesses{db: db}
}

// Create persists a new grant.
func (r *LocationAccesses) Create(ctx context.Context, a *models.LocationAccess) error {
	err := r.db.DB().WithContext(ctx).Create(a).Error
	if err != nil {
		return err
	}

	return nil
}

// GetByID looks up a grant by ID, returning ErrNotFound if none matches.
func (r *LocationAccesses) GetByID(ctx context.Context, id string) (*models.LocationAccess, error) {
	var a models.LocationAccess

	err := r.db.DB().WithContext(ctx).First(&a, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &a, nil
}

// ListByLocation returns every grant on one location.
func (r *LocationAccesses) ListByLocation(ctx context.Context, locationID string) ([]models.LocationAccess, error) {
	var accesses []models.LocationAccess

	err := r.db.DB().WithContext(ctx).
		Where("location_id = ?", locationID).
		Order("created_at").
		Find(&accesses).Error
	if err != nil {
		return nil, err
	}

	return accesses, nil
}

// Exists reports whether an identical grant (same location and same
// character/group target) is already present.
func (r *LocationAccesses) Exists(ctx context.Context, a *models.LocationAccess) (bool, error) {
	var count int64

	query := r.db.DB().WithContext(ctx).
		Model(&models.LocationAccess{}).
		Where("location_id = ?", a.LocationID)
	if a.CharacterID != nil {
		query = query.Where("character_id = ?", *a.CharacterID)
	}
	if a.GroupID != nil {
		query = query.Where("group_id = ?", *a.GroupID)
	}

	if err := query.Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

// Delete soft-deletes a grant.
func (r *LocationAccesses) Delete(ctx context.Context, a *models.LocationAccess) error {
	err := r.db.DB().WithContext(ctx).Delete(a).Error
	if err != nil {
		return err
	}

	return nil
}

// AccessibleLocationIDs returns the distinct locations reachable through any
// of the given characters or groups.
func (r *LocationAccesses) AccessibleLocationIDs(
	ctx context.Context, characterIDs, groupIDs []string,
) ([]string, error) {
	if len(characterIDs) == 0 && len(groupIDs) == 0 {
		return nil, nil
	}

	var ids []string

	err := r.db.DB().WithContext(ctx).
		Model(&models.LocationAccess{}).
		Where(r.targetConditions(characterIDs, groupIDs)).
		Distinct().
		Pluck("location_id", &ids).Error
	if err != nil {
		return nil, err
	}

	return ids, nil
}

// HasAccess reports whether any of the given characters or groups holds a
// grant on the location.
func (r *LocationAccesses) HasAccess(
	ctx context.Context, locationID string, characterIDs, groupIDs []string,
) (bool, error) {
	if len(characterIDs) == 0 && len(groupIDs) == 0 {
		return false, nil
	}

	var count int64

	err := r.db.DB().WithContext(ctx).
		Model(&models.LocationAccess{}).
		Where("location_id = ?", locationID).
		Where(r.targetConditions(characterIDs, groupIDs)).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// targetConditions builds the "granted to one of these characters or groups"
// clause shared by the access queries.
func (r *LocationAccesses) targetConditions(characterIDs, groupIDs []string) *gorm.DB {
	db := r.db.DB()
	switch {
	case len(characterIDs) > 0 && len(groupIDs) > 0:
		return db.Where("character_id IN ?", characterIDs).Or("group_id IN ?", groupIDs)
	case len(characterIDs) > 0:
		return db.Where("character_id IN ?", characterIDs)
	default:
		return db.Where("group_id IN ?", groupIDs)
	}
}
