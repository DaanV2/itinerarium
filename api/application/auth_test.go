package application_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/stretchr/testify/assert"
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
	assert.Equal(t, userID, requester.UserID())
	assert.True(t, requester.IsGM(), "expected the requester to be a GM")
}

func TestAuthService_Authenticate_RejectsGarbageToken(t *testing.T) {
	auth, _, _ := newTestAuthService(t)

	_, err := auth.Authenticate(t.Context(), "not-a-token")
	require.ErrorIs(t, err, application.ErrUnauthenticated)
}

func TestAuthService_Authenticate_RejectsUnknownSubject(t *testing.T) {
	auth, tokens, _ := newTestAuthService(t)

	token, err := tokens.Issue("does-not-exist")
	require.NoError(t, err)

	_, err = auth.Authenticate(t.Context(), token)
	require.ErrorIs(t, err, application.ErrUnauthenticated)
}

func TestAuthService_Login(t *testing.T) {
	auth := newTestAuthServiceWithPassword(t, "player@example.com", "hunter22hunter")

	user, token, err := auth.Login(t.Context(), "player@example.com", "hunter22hunter")
	require.NoError(t, err)
	assert.Equal(t, "player@example.com", user.Email)
	assert.NotEmpty(t, token, "expected a non-empty access token")
}

func TestAuthService_Login_RejectsWrongPassword(t *testing.T) {
	auth := newTestAuthServiceWithPassword(t, "player@example.com", "hunter22hunter")

	_, _, err := auth.Login(t.Context(), "player@example.com", "wrong-password")
	require.ErrorIs(t, err, application.ErrInvalidCredentials)
}

func TestAuthService_Login_RejectsUnknownEmail(t *testing.T) {
	auth := newTestAuthServiceWithPassword(t, "player@example.com", "hunter22hunter")

	_, _, err := auth.Login(t.Context(), "nobody@example.com", "hunter22hunter")
	require.ErrorIs(t, err, application.ErrInvalidCredentials)
}
