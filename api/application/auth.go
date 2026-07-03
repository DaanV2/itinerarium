package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"gorm.io/gorm"
)

// AuthService authenticates requests by validating access tokens and loading
// the Requester they identify.
type AuthService struct {
	tokens *authentication.TokenService
	users  *repositories.Users
}

// NewAuthService builds an AuthService.
func NewAuthService(tokens *authentication.TokenService, users *repositories.Users) *AuthService {
	return &AuthService{tokens: tokens, users: users}
}

// Authenticate validates a bearer token and returns the Requester it
// identifies. It fails with ErrUnauthenticated for any invalid, expired,
// revoked, or unrecognized-subject token.
func (s *AuthService) Authenticate(ctx context.Context, token string) (Requester, error) {
	claims, err := s.tokens.Parse(ctx, token)
	if err != nil {
		return nil, ErrUnauthenticated
	}

	user, err := s.users.GetByID(ctx, claims.Subject)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUnauthenticated
		}

		return nil, fmt.Errorf("loading requester: %w", err)
	}

	return UserRequester{User: user}, nil
}
