package repositories

import (
	"context"
	"errors"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
)

// Sessions provides access to play sessions and their participants.
type Sessions struct{ db *persistence.Database }

// NewSessions builds a Sessions repository.
func NewSessions(db *persistence.Database) *Sessions {
	return &Sessions{db: db}
}

// Create persists a new session.
func (r *Sessions) Create(ctx context.Context, s *models.Session) error {
	err := r.db.DB().WithContext(ctx).Create(s).Error
	if err != nil {
		return err
	}

	return nil
}

// GetByID looks up a session by ID with its current participants preloaded.
// It returns ErrNotFound if no session matches.
func (r *Sessions) GetByID(ctx context.Context, id string) (*models.Session, error) {
	var s models.Session

	err := r.db.DB().WithContext(ctx).Preload("Participants").First(&s, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &s, nil
}

// List returns every session with participants preloaded, ordered by name.
func (r *Sessions) List(ctx context.Context) ([]models.Session, error) {
	var sessions []models.Session

	err := r.db.DB().WithContext(ctx).Preload("Participants").Order("name").Find(&sessions).Error
	if err != nil {
		return nil, err
	}

	return sessions, nil
}

// Update persists changes to a session's own columns (not its participant
// list).
func (r *Sessions) Update(ctx context.Context, s *models.Session) error {
	err := r.db.DB().WithContext(ctx).Omit("Participants").Save(s).Error
	if err != nil {
		return err
	}

	return nil
}

// IsParticipant reports whether the character currently participates in the
// session.
func (r *Sessions) IsParticipant(ctx context.Context, sessionID, characterID string) (bool, error) {
	var count int64

	err := r.db.DB().WithContext(ctx).
		Table("session_participants").
		Where("session_id = ? AND character_id = ?", sessionID, characterID).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// AddParticipant adds the character to the session.
func (r *Sessions) AddParticipant(ctx context.Context, s *models.Session, c *models.Character) error {
	err := r.db.DB().WithContext(ctx).Model(s).Association("Participants").Append(c)
	if err != nil {
		return err
	}

	return nil
}

// RemoveParticipant removes the character from the session.
func (r *Sessions) RemoveParticipant(ctx context.Context, s *models.Session, c *models.Character) error {
	err := r.db.DB().WithContext(ctx).Model(s).Association("Participants").Delete(c)
	if err != nil {
		return err
	}

	return nil
}
