package application_test

import (
	"errors"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

func newTestCharactersEnv(t *testing.T) (*application.CharacterService, *repositories.Users) {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	if err != nil {
		t.Fatalf("persistence.New: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	users := repositories.NewUsers(db)
	characters := repositories.NewCharacters(db)

	return application.NewCharacterService(characters, users), users
}

func TestCharacterService_Create_ForSelf(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	c, err := svc.Create(ctx, playerRequester, "", "Aria")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.UserID != playerRequester.UserID() {
		t.Fatalf("UserID = %q, want %q", c.UserID, playerRequester.UserID())
	}
	if c.CurrentGameDay != 0 {
		t.Fatalf("CurrentGameDay = %d, want 0", c.CurrentGameDay)
	}
}

func TestCharacterService_Create_MultiplePerUser(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	if _, err := svc.Create(ctx, playerRequester, "", "Aria"); err != nil {
		t.Fatalf("Create (first): %v", err)
	}
	if _, err := svc.Create(ctx, playerRequester, "", "Beren"); err != nil {
		t.Fatalf("Create (second): %v", err)
	}

	characters, err := svc.List(ctx, playerRequester)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(characters) != 2 {
		t.Fatalf("List returned %d characters, want 2", len(characters))
	}
}

func TestCharacterService_Create_RejectsEmptyName(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	_, err := svc.Create(ctx, playerRequester, "", "")
	if !errors.Is(err, application.ErrInvalidName) {
		t.Fatalf("Create(empty name) = %v, want ErrInvalidName", err)
	}
}

func TestCharacterService_Create_PlayerCannotCreateForAnotherUser(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	_, err := svc.Create(ctx, playerRequester, "someone-else", "Aria")
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("Create(other owner) = %v, want ErrForbidden", err)
	}
}

func TestCharacterService_Create_GMForUnknownUser(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	_, err := svc.Create(ctx, gmRequester, "does-not-exist", "Aria")
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Create(unknown owner) = %v, want ErrNotFound", err)
	}
}

func TestCharacterService_Create_GMForExistingUser(t *testing.T) {
	svc, users := newTestCharactersEnv(t)
	ctx := t.Context()

	owner := &models.User{Email: "owner@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	if err := users.Create(ctx, owner); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	c, err := svc.Create(ctx, gmRequester, owner.ID, "Aria")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.UserID != owner.ID {
		t.Fatalf("UserID = %q, want %q", c.UserID, owner.ID)
	}
}

func TestCharacterService_List_PlayerSeesOnlyOwn(t *testing.T) {
	svc, users := newTestCharactersEnv(t)
	ctx := t.Context()

	other := &models.User{Email: "other@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	if err := users.Create(ctx, other); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	if _, err := svc.Create(ctx, playerRequester, "", "Aria"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx, gmRequester, other.ID, "Beren"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	characters, err := svc.List(ctx, playerRequester)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(characters) != 1 {
		t.Fatalf("List returned %d characters, want 1", len(characters))
	}
}

func TestCharacterService_List_GMSeesAll(t *testing.T) {
	svc, users := newTestCharactersEnv(t)
	ctx := t.Context()

	other := &models.User{Email: "other@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	if err := users.Create(ctx, other); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	if _, err := svc.Create(ctx, playerRequester, "", "Aria"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx, gmRequester, other.ID, "Beren"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	characters, err := svc.List(ctx, gmRequester)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(characters) != 2 {
		t.Fatalf("List returned %d characters, want 2", len(characters))
	}
}

func TestCharacterService_Get_HidesOtherOwnersCharacter(t *testing.T) {
	svc, users := newTestCharactersEnv(t)
	ctx := t.Context()

	other := &models.User{Email: "other@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	if err := users.Create(ctx, other); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	c, err := svc.Create(ctx, gmRequester, other.ID, "Beren")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = svc.Get(ctx, playerRequester, c.ID)
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Get(other owner's character) = %v, want ErrNotFound", err)
	}
}

func TestCharacterService_Get_OwnerCanSeeOwnCharacter(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	c, err := svc.Create(ctx, playerRequester, "", "Aria")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.Get(ctx, playerRequester, c.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != c.ID {
		t.Fatalf("Get returned %q, want %q", got.ID, c.ID)
	}
}

func TestCharacterService_Get_UnknownCharacter(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	_, err := svc.Get(ctx, gmRequester, "does-not-exist")
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Get(unknown) = %v, want ErrNotFound", err)
	}
}

func TestCharacterService_Update_OwnerCanRename(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	c, err := svc.Create(ctx, playerRequester, "", "Aria")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newName := "Aria the Bold"

	updated, err := svc.Update(ctx, playerRequester, c.ID, &newName, nil)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != newName {
		t.Fatalf("Name = %q, want %q", updated.Name, newName)
	}
}

func TestCharacterService_Update_PlayerCannotSetGameDay(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	c, err := svc.Create(ctx, playerRequester, "", "Aria")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	day := 5

	_, err = svc.Update(ctx, playerRequester, c.ID, nil, &day)
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("Update(player sets game day) = %v, want ErrForbidden", err)
	}
}

func TestCharacterService_Update_GMCanSetGameDay(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	c, err := svc.Create(ctx, playerRequester, "", "Aria")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	day := 5

	updated, err := svc.Update(ctx, gmRequester, c.ID, nil, &day)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.CurrentGameDay != day {
		t.Fatalf("CurrentGameDay = %d, want %d", updated.CurrentGameDay, day)
	}
}

func TestCharacterService_Update_RejectsNegativeGameDay(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	c, err := svc.Create(ctx, playerRequester, "", "Aria")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	day := -1

	_, err = svc.Update(ctx, gmRequester, c.ID, nil, &day)
	if !errors.Is(err, application.ErrInvalidGameDay) {
		t.Fatalf("Update(negative game day) = %v, want ErrInvalidGameDay", err)
	}
}

func TestCharacterService_Update_OtherOwnersCharacterIsHidden(t *testing.T) {
	svc, users := newTestCharactersEnv(t)
	ctx := t.Context()

	other := &models.User{Email: "other@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	if err := users.Create(ctx, other); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	c, err := svc.Create(ctx, gmRequester, other.ID, "Beren")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newName := "Hijacked"

	_, err = svc.Update(ctx, playerRequester, c.ID, &newName, nil)
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Update(other owner's character) = %v, want ErrNotFound", err)
	}
}
