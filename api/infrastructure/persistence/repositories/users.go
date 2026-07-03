package repositories

import (
	"context"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
)

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

// GetByEmail looks up a user by email.
func (r *Users) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User

	err := r.db.DB().WithContext(ctx).First(&u, "email = ?", email).Error
	if err != nil {
		return nil, err
	}

	return &u, nil
}
