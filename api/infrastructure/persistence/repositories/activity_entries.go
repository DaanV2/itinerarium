package repositories

import (
	"context"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
)

// ActivityEntries provides access to the append-only campaign event log.
// Entries are only ever created — never updated or deleted.
type ActivityEntries struct{ db *persistence.Database }

// NewActivityEntries builds an ActivityEntries repository.
func NewActivityEntries(db *persistence.Database) *ActivityEntries {
	return &ActivityEntries{db: db}
}

// Create appends one event to the log, including any announcement targets it
// carries.
func (r *ActivityEntries) Create(ctx context.Context, entry *models.ActivityEntry) error {
	err := r.db.DB().WithContext(ctx).Create(entry).Error
	if err != nil {
		return err
	}

	return nil
}

// createEntries appends activity entries inside an already-open transaction —
// the hook other repositories use to record events atomically with the change
// they describe (the M2 membership precedent, extended to every tracked
// event). Nil entries are skipped so callers can pass "no event" unconditionally.
func createEntries(tx *gorm.DB, entries []*models.ActivityEntry) error {
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if err := tx.Create(entry).Error; err != nil {
			return err
		}
	}

	return nil
}

// ListByEntity returns every event recorded against one entity, oldest first.
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

// ListAll returns every event with announcement targets preloaded, newest
// first — the GM-wide view (GMs see all activity regardless of game day).
func (r *ActivityEntries) ListAll(ctx context.Context) ([]models.ActivityEntry, error) {
	var entries []models.ActivityEntry

	err := r.db.DB().WithContext(ctx).
		Preload("Targets").
		Order("game_day DESC, created_at DESC").
		Find(&entries).Error
	if err != nil {
		return nil, err
	}

	return entries, nil
}

// FeedFilter describes what one character may see in their activity feed. The
// application layer resolves the character's reachable scopes and announcement
// identity; this repository only translates them into a query.
type FeedFilter struct {
	// MaxGameDay gates every entry: only events at or before it surface.
	MaxGameDay int
	// Scopes the character has access to (the normal visibility path).
	GroupIDs      []string
	LocationIDs   []string
	RepositoryIDs []string
	// Announcement identity (the announced visibility path): the character
	// itself plus the groups it belongs to.
	CharacterID    string
	TargetGroupIDs []string
}

// ListFeed returns the entries visible to one character, newest first: entries
// whose scope the character can access, plus announced entries targeted at the
// character (directly, via a group, or publicly) — all gated by MaxGameDay.
// Announcement targets are deliberately not loaded; they are GM-only detail.
func (r *ActivityEntries) ListFeed(ctx context.Context, filter *FeedFilter) ([]models.ActivityEntry, error) {
	var entries []models.ActivityEntry

	err := r.db.DB().WithContext(ctx).
		Where("game_day <= ?", filter.MaxGameDay).
		Where(r.scopeConditions(filter).Or(r.announcedConditions(filter))).
		Order("game_day DESC, created_at DESC").
		Find(&entries).Error
	if err != nil {
		return nil, err
	}

	return entries, nil
}

// scopeConditions builds the normal-path clause: the entry's scope is one the
// character can access. With no reachable scopes it matches nothing.
func (r *ActivityEntries) scopeConditions(filter *FeedFilter) *gorm.DB {
	// Seed with a never-true condition so the OR chain below stays valid even
	// when every scope list is empty.
	query := r.db.DB().Where("1 = 0")

	if len(filter.GroupIDs) > 0 {
		query = query.Or("scope_type = ? AND scope_id IN ?", models.ActivityScopeGroup, filter.GroupIDs)
	}
	if len(filter.LocationIDs) > 0 {
		query = query.Or("scope_type = ? AND scope_id IN ?", models.ActivityScopeLocation, filter.LocationIDs)
	}
	if len(filter.RepositoryIDs) > 0 {
		query = query.Or("scope_type = ? AND scope_id IN ?", models.ActivityScopeRepository, filter.RepositoryIDs)
	}

	return query
}

// announcedConditions builds the announced-path clause: the entry is announced
// publicly, or explicitly targets the character or one of its groups.
func (r *ActivityEntries) announcedConditions(filter *FeedFilter) *gorm.DB {
	db := r.db.DB()

	targets := db.Model(&models.ActivityTarget{}).Select("activity_entry_id")
	if len(filter.TargetGroupIDs) > 0 {
		targets = targets.Where("character_id = ? OR group_id IN ?", filter.CharacterID, filter.TargetGroupIDs)
	} else {
		targets = targets.Where("character_id = ?", filter.CharacterID)
	}

	return db.Where("announced = ?", true).
		Where(db.Where("announced_public = ?", true).Or("activity_entries.id IN (?)", targets))
}
