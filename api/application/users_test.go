package application_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRequester is a minimal application.Requester for service tests that
// don't need a persisted user behind the caller.
type fakeRequester struct {
	id string
	gm bool
}

func (r fakeRequester) UserID() string { return r.id }

func (r fakeRequester) IsGM() bool { return r.gm }

var (
	gmRequester     = fakeRequester{id: "gm-1", gm: true}
	playerRequester = fakeRequester{id: "player-1", gm: false}
)

func newTestUsersRepo(t *testing.T) *repositories.Users {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err)
	err = db.Migrate()
	require.NoError(t, err)

	return repositories.NewUsers(db)
}

func TestUserService_CreateAccount_RequiresGM(t *testing.T) {
	svc := application.NewUserService(newTestUsersRepo(t))
	ctx := t.Context()

	_, _, err := svc.CreateAccount(ctx, playerRequester, "new@example.com", models.RolePlayer)
	require.ErrorIs(t, err, application.ErrForbidden)
}

func TestUserService_CreateAccount(t *testing.T) {
	svc := application.NewUserService(newTestUsersRepo(t))
	ctx := t.Context()

	user, password, err := svc.CreateAccount(ctx, gmRequester, "player@example.com", models.RolePlayer)
	require.NoError(t, err)
	assert.Equal(t, models.RolePlayer, user.Role, "Role = %q, want player", user.Role)
	assert.Greater(t, len(password), 8, "temporary password length = %d, want > 8", len(password))
}

func TestUserService_CreateAccount_RejectsDuplicateEmail(t *testing.T) {
	svc := application.NewUserService(newTestUsersRepo(t))
	ctx := t.Context()

	_, _, err := svc.CreateAccount(ctx, gmRequester, "dup@example.com", models.RolePlayer)
	require.NoError(t, err)

	_, _, err = svc.CreateAccount(ctx, gmRequester, "dup@example.com", models.RoleGM)
	require.ErrorIs(t, err, application.ErrEmailTaken)
}

func TestUserService_CreateAccount_ValidatesInput(t *testing.T) {
	svc := application.NewUserService(newTestUsersRepo(t))
	ctx := t.Context()

	tests := []struct {
		name    string
		email   string
		role    models.Role
		wantErr error
	}{
		{"invalid email", "not-an-email", models.RolePlayer, application.ErrInvalidEmail},
		{"invalid role", "valid@example.com", models.Role("wizard"), application.ErrInvalidRole},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := svc.CreateAccount(ctx, gmRequester, tt.email, tt.role)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestUserService_ListAccounts_RequiresGM(t *testing.T) {
	svc := application.NewUserService(newTestUsersRepo(t))
	ctx := t.Context()

	_, err := svc.ListAccounts(ctx, playerRequester)
	require.ErrorIs(t, err, application.ErrForbidden)
}

func TestUserService_ListAccounts(t *testing.T) {
	svc := application.NewUserService(newTestUsersRepo(t))
	ctx := t.Context()

	_, _, err := svc.CreateAccount(ctx, gmRequester, "one@example.com", models.RolePlayer)
	require.NoError(t, err)

	users, err := svc.ListAccounts(ctx, gmRequester)
	require.NoError(t, err)
	assert.Len(t, users, 1)
}

func TestUserService_ResetPassword_RequiresGM(t *testing.T) {
	svc := application.NewUserService(newTestUsersRepo(t))
	ctx := t.Context()

	user, _, err := svc.CreateAccount(ctx, gmRequester, "player@example.com", models.RolePlayer)
	require.NoError(t, err)

	_, err = svc.ResetPassword(ctx, playerRequester, user.ID)
	require.ErrorIs(t, err, application.ErrForbidden)
}

func TestUserService_ResetPassword(t *testing.T) {
	svc := application.NewUserService(newTestUsersRepo(t))
	ctx := t.Context()

	user, originalPassword, err := svc.CreateAccount(ctx, gmRequester, "player@example.com", models.RolePlayer)
	require.NoError(t, err)

	newPassword, err := svc.ResetPassword(ctx, gmRequester, user.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, newPassword, "expected a non-empty temporary password")
	assert.NotEqual(t, originalPassword, newPassword, "expected the reset password to differ from the original")
}

func TestUserService_ResetPassword_UnknownAccount(t *testing.T) {
	svc := application.NewUserService(newTestUsersRepo(t))
	ctx := t.Context()

	_, err := svc.ResetPassword(ctx, gmRequester, "does-not-exist")
	require.ErrorIs(t, err, application.ErrNotFound)
}
