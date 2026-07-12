package repositories

import (
	"context"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
)

// ActivityEntries provides access to the append-only campaign event log.
// Entries are only ever created — never updated or deleted.
type ActivityEntries struct{ db *persistence.Database }

// NewActivityEntries builds an ActivityEntries repository.
func NewActivityEntries(db *persistence.Database) *ActivityEntries {
	return &ActivityEntries{db: db}
}

// Create appends one event to the log.
func (r *ActivityEntries) Create(ctx context.Context, entry *models.ActivityEntry) error {
	err := r.db.DB().WithContext(ctx).Create(entry).Error
	if err != nil {
		return err
	}

	return nil
}

// ListByEntity returns every event recorded against one entity, oldest first.
// The M5 activity feed adds game-day and access filtering on top of this.
func (r *ActivityEntries) ListByEntity(
	ctx context.Context, entityType, entityID string,
) ([]models.ActivityEntry, error) {
	var entries []models.ActivityEntry

	err := r.db.DB().WithContext(ctx).
		Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Order("created_at").
		Find(&entries).Error
	if err != nil {
		return nil, err
	}

	return entries, nil
}
