package models

import "time"

// RevokedToken marks a JWT JTI as revoked (logout, credential reset) until
// its original expiry. Once ExpiresAt has passed the JWT itself is already
// rejected by signature/expiry validation, so the row is safe to purge.
type RevokedToken struct {
	Model
	JTI       string    `gorm:"uniqueIndex;not null" json:"jti"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
}
