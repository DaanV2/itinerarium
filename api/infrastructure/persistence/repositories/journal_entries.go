package repositories

import (
	"context"
	"errors"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
)

// JournalEntries provides access to per-character journal entries.
type JournalEntries struct{ db *persistence.Database }

// NewJournalEntries builds a JournalEntries repository.
func NewJournalEntries(db *persistence.Database) *JournalEntries {
	return &JournalEntries{db: db}
}

// Create persists a new journal entry.
func (r *JournalEntries) Create(ctx context.Context, e *models.JournalEntry) error {
	err := r.db.DB().WithContext(ctx).Create(e).Error
	if err != nil {
		return err
	}

	return nil
}

// GetByID looks up a journal entry by ID. It returns ErrNotFound if no entry
// matches.
func (r *JournalEntries) GetByID(ctx context.Context, id string) (*models.JournalEntry, error) {
	var e models.JournalEntry

	err := r.db.DB().WithContext(ctx).First(&e, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &e, nil
}

// ListByCharacter returns every journal entry for a character, ordered by
// game day.
func (r *JournalEntries) ListByCharacter(ctx context.Context, characterID string) ([]models.JournalEntry, error) {
	var entries []models.JournalEntry

	err := r.db.DB().WithContext(ctx).
		Where("character_id = ?", characterID).
		Order("game_day").
		Find(&entries).Error
	if err != nil {
		return nil, err
	}

	return entries, nil
}

// Update persists changes to an existing journal entry.
func (r *JournalEntries) Update(ctx context.Context, e *models.JournalEntry) error {
	err := r.db.DB().WithContext(ctx).Save(e).Error
	if err != nil {
		return err
	}

	return nil
}
