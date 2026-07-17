package application_test

import (
	"errors"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/stretchr/testify/require"
)

func newTestAuthServiceWithPassword(t *testing.T, email, password string) *application.AuthService {
	t.Helper()

	users := newTestUsersRepo(t)
	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(t.TempDir()))
	require.NoError(t, err)

	tokens := authentication.NewTokenService(keys, noopRevocationStore{})

	hash, err := authentication.HashPassword(password)
	require.NoError(t, err)

	user := &models.User{Email: email, PasswordHash: hash, Role: models.RolePlayer}
	err = users.Create(t.Context(), user)
	require.NoError(t, err)

	return application.NewAuthService(tokens, users)
}

func newTestAuthService(t *testing.T) (*application.AuthService, *authentication.TokenService, string) {
	t.Helper()

	users := newTestUsersRepo(t)
	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(t.TempDir()))
	require.NoError(t, err)

	tokens := authentication.NewTokenService(keys, noopRevocationStore{})

	user := &models.User{Email: "gm@example.com", PasswordHash: "hash", Role: models.RoleGM}
	err = users.Create(t.Context(), user)
	require.NoError(t, err)

	return application.NewAuthService(tokens, users), tokens, user.ID
}

func TestAuthService_Authenticate(t *testing.T) {
	auth, tokens, userID := newTestAuthService(t)
	ctx := t.Context()

	token, err := tokens.Issue(userID)
	require.NoError(t, err)

	requester, err := auth.Authenticate(ctx, token)
	require.NoError(t, err)
	if requester.UserID() != userID {
		t.Fatalf("UserID = %q, want %q", requester.UserID(), userID)
	}
	if !requester.IsGM() {
		t.Fatal("expected the requester to be a GM")
	}
}

func TestAuthService_Authenticate_RejectsGarbageToken(t *testing.T) {
	auth, _, _ := newTestAuthService(t)

	_, err := auth.Authenticate(t.Context(), "not-a-token")
	if !errors.Is(err, application.ErrUnauthenticated) {
		t.Fatalf("Authenticate(garbage) = %v, want ErrUnauthenticated", err)
	}
}

func TestAuthService_Authenticate_RejectsUnknownSubject(t *testing.T) {
	auth, tokens, _ := newTestAuthService(t)

	token, err := tokens.Issue("does-not-exist")
	require.NoError(t, err)

	_, err = auth.Authenticate(t.Context(), token)
	if !errors.Is(err, application.ErrUnauthenticated) {
		t.Fatalf("Authenticate(unknown subject) = %v, want ErrUnauthenticated", err)
	}
}

func TestAuthService_Login(t *testing.T) {
	auth := newTestAuthServiceWithPassword(t, "player@example.com", "hunter22hunter")

	user, token, err := auth.Login(t.Context(), "player@example.com", "hunter22hunter")
	require.NoError(t, err)
	if user.Email != "player@example.com" {
		t.Fatalf("Email = %q, want player@example.com", user.Email)
	}
	if token == "" {
		t.Fatal("expected a non-empty access token")
	}
}

func TestAuthService_Login_RejectsWrongPassword(t *testing.T) {
	auth := newTestAuthServiceWithPassword(t, "player@example.com", "hunter22hunter")

	_, _, err := auth.Login(t.Context(), "player@example.com", "wrong-password")
	if !errors.Is(err, application.ErrInvalidCredentials) {
		t.Fatalf("Login(wrong password) = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthService_Login_RejectsUnknownEmail(t *testing.T) {
	auth := newTestAuthServiceWithPassword(t, "player@example.com", "hunter22hunter")

	_, _, err := auth.Login(t.Context(), "nobody@example.com", "hunter22hunter")
	if !errors.Is(err, application.ErrInvalidCredentials) {
		t.Fatalf("Login(unknown email) = %v, want ErrInvalidCredentials", err)
	}
}
