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
