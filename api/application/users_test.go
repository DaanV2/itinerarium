package application_test

import (
	"errors"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
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
	if err != nil {
		t.Fatalf("persistence.New: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	return repositories.NewUsers(db)
}

func TestUserService_CreateAccount_RequiresGM(t *testing.T) {
	svc := application.NewUserService(newTestUsersRepo(t))
	ctx := t.Context()

	_, _, err := svc.CreateAccount(ctx, playerRequester, "new@example.com", models.RolePlayer)
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("CreateAccount(player) = %v, want ErrForbidden", err)
	}
}

func TestUserService_CreateAccount(t *testing.T) {
	svc := application.NewUserService(newTestUsersRepo(t))
	ctx := t.Context()

	user, password, err := svc.CreateAccount(ctx, gmRequester, "player@example.com", models.RolePlayer)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if user.Role != models.RolePlayer {
		t.Fatalf("Role = %q, want player", user.Role)
	}
	if len(password) < 8 {
		t.Fatalf("temporary password %q is shorter than the minimum length", password)
	}
}

func TestUserService_CreateAccount_RejectsDuplicateEmail(t *testing.T) {
	svc := application.NewUserService(newTestUsersRepo(t))
	ctx := t.Context()

	if _, _, err := svc.CreateAccount(ctx, gmRequester, "dup@example.com", models.RolePlayer); err != nil {
		t.Fatalf("CreateAccount (first): %v", err)
	}

	_, _, err := svc.CreateAccount(ctx, gmRequester, "dup@example.com", models.RoleGM)
	if !errors.Is(err, application.ErrEmailTaken) {
		t.Fatalf("CreateAccount (duplicate) = %v, want ErrEmailTaken", err)
	}
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
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CreateAccount(%q, %q) = %v, want %v", tt.email, tt.role, err, tt.wantErr)
			}
		})
	}
}

func TestUserService_ListAccounts_RequiresGM(t *testing.T) {
	svc := application.NewUserService(newTestUsersRepo(t))
	ctx := t.Context()

	_, err := svc.ListAccounts(ctx, playerRequester)
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("ListAccounts(player) = %v, want ErrForbidden", err)
	}
}

func TestUserService_ListAccounts(t *testing.T) {
	svc := application.NewUserService(newTestUsersRepo(t))
	ctx := t.Context()

	if _, _, err := svc.CreateAccount(ctx, gmRequester, "one@example.com", models.RolePlayer); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	users, err := svc.ListAccounts(ctx, gmRequester)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("ListAccounts returned %d users, want 1", len(users))
	}
}

func TestUserService_ResetPassword_RequiresGM(t *testing.T) {
	svc := application.NewUserService(newTestUsersRepo(t))
	ctx := t.Context()

	user, _, err := svc.CreateAccount(ctx, gmRequester, "player@example.com", models.RolePlayer)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	_, err = svc.ResetPassword(ctx, playerRequester, user.ID)
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("ResetPassword(player) = %v, want ErrForbidden", err)
	}
}

func TestUserService_ResetPassword(t *testing.T) {
	svc := application.NewUserService(newTestUsersRepo(t))
	ctx := t.Context()

	user, originalPassword, err := svc.CreateAccount(ctx, gmRequester, "player@example.com", models.RolePlayer)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	newPassword, err := svc.ResetPassword(ctx, gmRequester, user.ID)
	if err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	if newPassword == "" {
		t.Fatal("expected a non-empty temporary password")
	}
	if newPassword == originalPassword {
		t.Fatal("expected the reset password to differ from the original")
	}
}

func TestUserService_ResetPassword_UnknownAccount(t *testing.T) {
	svc := application.NewUserService(newTestUsersRepo(t))
	ctx := t.Context()

	_, err := svc.ResetPassword(ctx, gmRequester, "does-not-exist")
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("ResetPassword(unknown) = %v, want ErrNotFound", err)
	}
}
