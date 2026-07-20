package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// ErrInvalidCredentials is returned when a login attempt has an unknown
// email or a wrong password. Both cases take this same path so the response
// never reveals whether an email is registered.
var ErrInvalidCredentials = serviceErr(KindUnauthenticated, "invalid email or password")

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

// Login validates an email + password pair and, on success, returns the
// account plus a freshly issued access token. It fails with
// ErrInvalidCredentials for an unknown email or wrong password.
func (s *AuthService) Login(ctx context.Context, email, password string) (*models.User, string, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, "", ErrInvalidCredentials
		}

		return nil, "", fmt.Errorf("loading account: %w", err)
	}

	if !authentication.VerifyPassword(user.PasswordHash, password) {
		return nil, "", ErrInvalidCredentials
	}

	token, err := s.tokens.Issue(user.ID)
	if err != nil {
		return nil, "", fmt.Errorf("issuing token: %w", err)
	}

	return user, token, nil
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
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrUnauthenticated
		}

		return nil, fmt.Errorf("loading requester: %w", err)
	}

	return UserRequester{User: user}, nil
}
