package repositories

import (
	"context"
	"time"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm/clause"
)

// RevokedTokens persists revoked JWT JTIs, implementing
// authentication.RevocationStore.
type RevokedTokens struct{ db *persistence.Database }

// NewRevokedTokens builds a RevokedTokens repository.
func NewRevokedTokens(db *persistence.Database) *RevokedTokens {
	return &RevokedTokens{db: db}
}

// Revoke records a JTI as revoked until expiresAt. Revoking the same JTI
// twice is a no-op.
func (r *RevokedTokens) Revoke(ctx context.Context, jti string, expiresAt time.Time) error {
	token := models.RevokedToken{JTI: jti, ExpiresAt: expiresAt}

	err := r.db.DB().WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "jti"}}, DoNothing: true}).
		Create(&token).Error
	if err != nil {
		return err
	}

	return nil
}

// IsRevoked reports whether a JTI has been revoked.
func (r *RevokedTokens) IsRevoked(ctx context.Context, jti string) (bool, error) {
	var count int64

	err := r.db.DB().WithContext(ctx).Model(&models.RevokedToken{}).Where("jti = ?", jti).Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
