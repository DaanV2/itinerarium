package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

const minPasswordLength = 8

var (
	// ErrAlreadySetUp is returned once any account exists — the first-run
	// wizard runs exactly once per installation.
	ErrAlreadySetUp = errors.New("setup already completed")
	// ErrInvalidEmail is returned for an empty or obviously malformed email.
	ErrInvalidEmail = errors.New("invalid email")
	// ErrInvalidPassword is returned when the password is too short.
	ErrInvalidPassword = fmt.Errorf("password must be at least %d characters", minPasswordLength)
)

// SetupService bootstraps a fresh installation with its first (GM) account.
type SetupService struct {
	users  *repositories.Users
	tokens *authentication.TokenService
}

// NewSetupService builds a SetupService.
func NewSetupService(users *repositories.Users, tokens *authentication.TokenService) *SetupService {
	return &SetupService{users: users, tokens: tokens}
}

// NeedsSetup reports whether the installation has zero accounts yet.
func (s *SetupService) NeedsSetup(ctx context.Context) (bool, error) {
	count, err := s.users.Count(ctx)
	if err != nil {
		return false, fmt.Errorf("counting users: %w", err)
	}

	return count == 0, nil
}

// CreateInitialGM creates the installation's first account, always at GM
// rank, and returns it along with a signed-in access token. It fails with
// ErrAlreadySetUp once any account already exists.
func (s *SetupService) CreateInitialGM(ctx context.Context, email, password string) (*models.User, string, error) {
	if email == "" || !strings.Contains(email, "@") {
		return nil, "", ErrInvalidEmail
	}
	if len(password) < minPasswordLength {
		return nil, "", ErrInvalidPassword
	}

	needsSetup, err := s.NeedsSetup(ctx)
	if err != nil {
		return nil, "", err
	}
	if !needsSetup {
		return nil, "", ErrAlreadySetUp
	}

	hash, err := authentication.HashPassword(password)
	if err != nil {
		return nil, "", fmt.Errorf("hashing password: %w", err)
	}

	user := &models.User{Email: email, PasswordHash: hash, Role: models.RoleGM}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, "", fmt.Errorf("creating initial account: %w", err)
	}

	token, err := s.tokens.Issue(user.ID)
	if err != nil {
		return nil, "", fmt.Errorf("issuing token: %w", err)
	}

	return user, token, nil
}
