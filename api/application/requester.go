package application

import "github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"

// Requester identifies the authenticated caller of a service method. Every
// service that enforces permission rules accepts one instead of trusting
// caller-supplied identity.
type Requester interface {
	UserID() string
	IsGM() bool
}

// UserRequester adapts a persisted User to Requester.
type UserRequester struct {
	User *models.User
}

// UserID returns the requester's account ID.
func (r UserRequester) UserID() string { return r.User.ID }

// IsGM reports whether the requester holds the GM role.
func (r UserRequester) IsGM() bool { return r.User.Role == models.RoleGM }
