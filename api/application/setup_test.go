package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/stretchr/testify/require"
)

// noopRevocationStore never revokes anything; the setup flow only issues
// tokens, it never needs to check or record revocation.
type noopRevocationStore struct{}

func (noopRevocationStore) Revoke(context.Context, string, time.Time) error { return nil }

func (noopRevocationStore) IsRevoked(context.Context, string) (bool, error) { return false, nil }

func newTestSetupService(t *testing.T) *application.SetupService {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err)
	err = db.Migrate()
	require.NoError(t, err)

	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(t.TempDir()))
	require.NoError(t, err)

	tokens := authentication.NewTokenService(keys, noopRevocationStore{})
	users := repositories.NewUsers(db)

	return application.NewSetupService(users, tokens)
}

func TestSetupService_CreateInitialGM(t *testing.T) {
	svc := newTestSetupService(t)
	ctx := t.Context()

	needsSetup, err := svc.NeedsSetup(ctx)
	require.NoError(t, err)
	if !needsSetup {
		t.Fatal("expected a fresh installation to need setup")
	}

	user, token, err := svc.CreateInitialGM(ctx, "gm@example.com", "hunter22hunter")
	require.NoError(t, err)
	if user.Role != models.RoleGM {
		t.Fatalf("Role = %q, want gm", user.Role)
	}
	if token == "" {
		t.Fatal("expected a non-empty access token")
	}

	needsSetup, err = svc.NeedsSetup(ctx)
	require.NoError(t, err)
	if needsSetup {
		t.Fatal("expected setup to be complete after creating the initial account")
	}
}

func TestSetupService_CreateInitialGM_RefusesOnceSetUp(t *testing.T) {
	svc := newTestSetupService(t)
	ctx := t.Context()

	if _, _, err := svc.CreateInitialGM(ctx, "gm@example.com", "hunter22hunter"); err != nil {
		t.Fatalf("CreateInitialGM (first): %v", err)
	}

	_, _, err := svc.CreateInitialGM(ctx, "second@example.com", "hunter22hunter")
	if !errors.Is(err, application.ErrAlreadySetUp) {
		t.Fatalf("CreateInitialGM (second) = %v, want ErrAlreadySetUp", err)
	}
}

func TestSetupService_CreateInitialGM_ValidatesInput(t *testing.T) {
	svc := newTestSetupService(t)
	ctx := t.Context()

	tests := []struct {
		name     string
		email    string
		password string
		wantErr  error
	}{
		{"empty email", "", "hunter22hunter", application.ErrInvalidEmail},
		{"malformed email", "not-an-email", "hunter22hunter", application.ErrInvalidEmail},
		{"short password", "gm@example.com", "short", application.ErrInvalidPassword},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := svc.CreateInitialGM(ctx, tt.email, tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CreateInitialGM(%q, %q) = %v, want %v", tt.email, tt.password, err, tt.wantErr)
			}
		})
	}
}
