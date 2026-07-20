package application

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"

	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// temporaryPasswordBytes sizes the random temporary password generated for
// new accounts and password resets: 15 random bytes base32-encode to 24
// characters, well above minPasswordLength.
const temporaryPasswordBytes = 15

// ErrInvalidRole is returned when an account is created with a role other
// than gm or player.
var ErrInvalidRole = serviceErr(KindValidation, "invalid role")

// ErrEmailTaken is returned when creating an account with an email already
// in use.
var ErrEmailTaken = serviceErr(KindConflict, "email already in use")

// UserService manages accounts on behalf of a GM: creation and password
// resets. Both actions are GM-only — there is no self-registration and no
// SMTP-based reset flow.
type UserService struct {
	users *repositories.Users
}

// NewUserService builds a UserService.
func NewUserService(users *repositories.Users) *UserService {
	return &UserService{users: users}
}

// CreateAccount creates a new player or GM account with a random temporary
// password, which the GM hands out to the account holder out of band. Only a
// GM may call this.
func (s *UserService) CreateAccount(
	ctx context.Context, requester Requester, email string, role models.Role,
) (*models.User, string, error) {
	if !requester.IsGM() {
		return nil, "", ErrForbidden
	}
	if email == "" || !strings.Contains(email, "@") {
		return nil, "", ErrInvalidEmail
	}
	if role != models.RoleGM && role != models.RolePlayer {
		return nil, "", ErrInvalidRole
	}

	_, err := s.users.GetByEmail(ctx, email)
	if err == nil {
		return nil, "", ErrEmailTaken
	}
	if !errors.Is(err, repositories.ErrNotFound) {
		return nil, "", fmt.Errorf("checking existing account: %w", err)
	}

	password, hash, err := newTemporaryPassword()
	if err != nil {
		return nil, "", err
	}

	user := &models.User{Email: email, PasswordHash: hash, Role: role}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, "", fmt.Errorf("creating account: %w", err)
	}

	return user, password, nil
}

// ListAccounts returns every account for the admin panel. Only a GM may call
// this.
func (s *UserService) ListAccounts(ctx context.Context, requester Requester) ([]models.User, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}

	users, err := s.users.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing accounts: %w", err)
	}

	return users, nil
}

// ResetPassword issues a new random temporary password for an existing
// account, which the GM hands out to the account holder out of band. Only a
// GM may call this.
func (s *UserService) ResetPassword(ctx context.Context, requester Requester, userID string) (string, error) {
	if !requester.IsGM() {
		return "", ErrForbidden
	}

	_, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return "", ErrNotFound
		}

		return "", fmt.Errorf("loading account: %w", err)
	}

	password, hash, err := newTemporaryPassword()
	if err != nil {
		return "", err
	}

	if err := s.users.UpdatePasswordHash(ctx, userID, hash); err != nil {
		return "", fmt.Errorf("updating password: %w", err)
	}

	return password, nil
}

// newTemporaryPassword generates a random temporary password and its bcrypt
// hash.
func newTemporaryPassword() (password, hash string, err error) {
	buf := make([]byte, temporaryPasswordBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generating temporary password: %w", err)
	}

	password = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)

	hash, err = authentication.HashPassword(password)
	if err != nil {
		return "", "", fmt.Errorf("hashing temporary password: %w", err)
	}

	return password, hash, nil
}
