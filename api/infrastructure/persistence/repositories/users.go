package repositories

import (
	"context"
	"errors"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
)

// ErrNotFound is returned when no user matches the given lookup. It
// sanitizes gorm.ErrRecordNotFound so callers never need to import gorm.
var ErrNotFound = errors.New("user not found")

// Users provides access to user accounts.
type Users struct{ db *persistence.Database }

// NewUsers builds a Users repository.
func NewUsers(db *persistence.Database) *Users {
	return &Users{db: db}
}

// Count returns the number of accounts, used to detect a fresh install.
func (r *Users) Count(ctx context.Context) (int64, error) {
	var count int64

	err := r.db.DB().WithContext(ctx).Model(&models.User{}).Count(&count).Error
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Create persists a new user.
func (r *Users) Create(ctx context.Context, u *models.User) error {
	err := r.db.DB().WithContext(ctx).Create(u).Error
	if err != nil {
		return err
	}

	return nil
}

// GetByEmail looks up a user by email. It returns ErrNotFound if no user
// matches.
func (r *Users) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User

	err := r.db.DB().WithContext(ctx).First(&u, "email = ?", email).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &u, nil
}

// GetByID looks up a user by ID. It returns ErrNotFound if no user matches.
func (r *Users) GetByID(ctx context.Context, id string) (*models.User, error) {
	var u models.User

	err := r.db.DB().WithContext(ctx).First(&u, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &u, nil
}

// List returns every account, ordered by email, for the admin panel.
func (r *Users) List(ctx context.Context) ([]models.User, error) {
	var users []models.User

	err := r.db.DB().WithContext(ctx).Order("email").Find(&users).Error
	if err != nil {
		return nil, err
	}

	return users, nil
}

// UpdatePasswordHash sets a new password hash for the given user.
func (r *Users) UpdatePasswordHash(ctx context.Context, id, passwordHash string) error {
	err := r.db.DB().WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", id).
		Update("password_hash", passwordHash).Error
	if err != nil {
		return err
	}

	return nil
}
