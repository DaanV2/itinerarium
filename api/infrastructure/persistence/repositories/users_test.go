package repositories_test

import (
	"errors"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"gorm.io/gorm"
)

func TestUsers_CreateCountAndGetByEmail(t *testing.T) {
	repo := repositories.NewUsers(newTestDB(t))
	ctx := t.Context()

	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Fatalf("Count = %d, want 0 on a fresh database", count)
	}

	user := &models.User{Email: "gm@example.com", PasswordHash: "hash", Role: models.RoleGM}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if user.ID == "" {
		t.Fatal("expected Create to populate the generated ID")
	}

	count, err = repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Fatalf("Count = %d, want 1 after Create", count)
	}

	found, err := repo.GetByEmail(ctx, "gm@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if found.ID != user.ID {
		t.Fatalf("GetByEmail returned ID %q, want %q", found.ID, user.ID)
	}
}

func TestUsers_GetByEmail_NotFound(t *testing.T) {
	repo := repositories.NewUsers(newTestDB(t))

	_, err := repo.GetByEmail(t.Context(), "nobody@example.com")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetByEmail = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestUsers_GetByID(t *testing.T) {
	repo := repositories.NewUsers(newTestDB(t))
	ctx := t.Context()

	user := &models.User{Email: "player@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if found.Email != user.Email {
		t.Fatalf("GetByID returned email %q, want %q", found.Email, user.Email)
	}

	_, err = repo.GetByID(ctx, "not-an-id")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetByID(unknown) = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestUsers_List(t *testing.T) {
	repo := repositories.NewUsers(newTestDB(t))
	ctx := t.Context()

	if err := repo.Create(ctx, &models.User{Email: "b@example.com", PasswordHash: "hash", Role: models.RolePlayer}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Create(ctx, &models.User{Email: "a@example.com", PasswordHash: "hash", Role: models.RoleGM}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	users, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("List returned %d users, want 2", len(users))
	}
	if users[0].Email != "a@example.com" || users[1].Email != "b@example.com" {
		t.Fatalf("List not ordered by email: %+v", users)
	}
}

func TestUsers_UpdatePasswordHash(t *testing.T) {
	repo := repositories.NewUsers(newTestDB(t))
	ctx := t.Context()

	user := &models.User{Email: "player@example.com", PasswordHash: "old-hash", Role: models.RolePlayer}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.UpdatePasswordHash(ctx, user.ID, "new-hash"); err != nil {
		t.Fatalf("UpdatePasswordHash: %v", err)
	}

	found, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if found.PasswordHash != "new-hash" {
		t.Fatalf("PasswordHash = %q, want %q", found.PasswordHash, "new-hash")
	}
}
