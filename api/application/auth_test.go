package application_test

import (
	"errors"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
)

func newTestAuthService(t *testing.T) (*application.AuthService, *authentication.TokenService, string) {
	t.Helper()

	users := newTestUsersRepo(t)
	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(t.TempDir()))
	if err != nil {
		t.Fatalf("NewKeyStore: %v", err)
	}

	tokens := authentication.NewTokenService(keys, noopRevocationStore{})

	user := &models.User{Email: "gm@example.com", PasswordHash: "hash", Role: models.RoleGM}
	if err := users.Create(t.Context(), user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	return application.NewAuthService(tokens, users), tokens, user.ID
}

func TestAuthService_Authenticate(t *testing.T) {
	auth, tokens, userID := newTestAuthService(t)
	ctx := t.Context()

	token, err := tokens.Issue(userID)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	requester, err := auth.Authenticate(ctx, token)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
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
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	_, err = auth.Authenticate(t.Context(), token)
	if !errors.Is(err, application.ErrUnauthenticated) {
		t.Fatalf("Authenticate(unknown subject) = %v, want ErrUnauthenticated", err)
	}
}
