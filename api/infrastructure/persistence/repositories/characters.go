package repositories

import (
	"context"
	"errors"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
)

// Characters provides access to player characters.
type Characters struct{ db *persistence.Database }

// NewCharacters builds a Characters repository.
func NewCharacters(db *persistence.Database) *Characters {
	return &Characters{db: db}
}

// Create persists a new character.
func (r *Characters) Create(ctx context.Context, c *models.Character) error {
	err := r.db.DB().WithContext(ctx).Create(c).Error
	if err != nil {
		return err
	}

	return nil
}

// GetByID looks up a character by ID. It returns ErrNotFound if no
// character matches.
func (r *Characters) GetByID(ctx context.Context, id string) (*models.Character, error) {
	var c models.Character

	err := r.db.DB().WithContext(ctx).First(&c, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &c, nil
}

// ListByUser returns every character owned by the given user, ordered by
// name.
func (r *Characters) ListByUser(ctx context.Context, userID string) ([]models.Character, error) {
	var characters []models.Character

	err := r.db.DB().WithContext(ctx).Where("user_id = ?", userID).Order("name").Find(&characters).Error
	if err != nil {
		return nil, err
	}

	return characters, nil
}

// List returns every character, ordered by name, for GM-wide views.
func (r *Characters) List(ctx context.Context) ([]models.Character, error) {
	var characters []models.Character

	err := r.db.DB().WithContext(ctx).Order("name").Find(&characters).Error
	if err != nil {
		return nil, err
	}

	return characters, nil
}

// AnyWithGameDayAtLeast reports whether any character's current_game_day has
// reached the given day — used to decide whether a campaign-wide document
// counts as already revealed.
func (r *Characters) AnyWithGameDayAtLeast(ctx context.Context, day int) (bool, error) {
	var count int64

	err := r.db.DB().WithContext(ctx).Model(&models.Character{}).
		Where("current_game_day >= ?", day).Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// Update persists changes to an existing character.
func (r *Characters) Update(ctx context.Context, c *models.Character) error {
	err := r.db.DB().WithContext(ctx).Save(c).Error
	if err != nil {
		return err
	}

	return nil
}
