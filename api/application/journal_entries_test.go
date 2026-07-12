package application_test

import (
	"errors"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

func newTestJournalEntriesEnv(t *testing.T) (*application.JournalEntryService, *application.CharacterService) {
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
	entries := repositories.NewJournalEntries(db)

	characterSvc := application.NewCharacterService(characters, users, repositories.NewKnowledgeRepositories(db))

	return application.NewJournalEntryService(entries, characters), characterSvc
}

func TestJournalEntryService_Create_StampsCurrentGameDay(t *testing.T) {
	svc, characterSvc := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	c, err := characterSvc.Create(ctx, playerRequester, "", "Aria")
	if err != nil {
		t.Fatalf("Create character: %v", err)
	}

	day := 3

	if _, err := characterSvc.Update(ctx, gmRequester, c.ID, nil, &day); err != nil {
		t.Fatalf("Update character game day: %v", err)
	}

	e, err := svc.Create(ctx, playerRequester, c.ID, "Dear diary...")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if e.GameDay != day {
		t.Fatalf("GameDay = %d, want %d", e.GameDay, day)
	}
	if e.Content != "Dear diary..." {
		t.Fatalf("Content = %q, want %q", e.Content, "Dear diary...")
	}
}

func TestJournalEntryService_Create_RejectsEmptyContent(t *testing.T) {
	svc, characterSvc := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	c, err := characterSvc.Create(ctx, playerRequester, "", "Aria")
	if err != nil {
		t.Fatalf("Create character: %v", err)
	}

	_, err = svc.Create(ctx, playerRequester, c.ID, "")
	if !errors.Is(err, application.ErrInvalidContent) {
		t.Fatalf("Create(empty content) = %v, want ErrInvalidContent", err)
	}
}

func TestJournalEntryService_Create_OtherPlayerCannotWriteForForeignCharacter(t *testing.T) {
	svc, characterSvc := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	other := fakeRequester{id: "other-1", gm: false}

	c, err := characterSvc.Create(ctx, other, "", "Beren")
	if err != nil {
		t.Fatalf("Create character: %v", err)
	}

	_, err = svc.Create(ctx, playerRequester, c.ID, "Not mine to write")
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Create(foreign character) = %v, want ErrNotFound", err)
	}
}

func TestJournalEntryService_List_OwnerSeesOwnEntries(t *testing.T) {
	svc, characterSvc := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	c, err := characterSvc.Create(ctx, playerRequester, "", "Aria")
	if err != nil {
		t.Fatalf("Create character: %v", err)
	}

	if _, err := svc.Create(ctx, playerRequester, c.ID, "Entry one"); err != nil {
		t.Fatalf("Create entry: %v", err)
	}

	entries, err := svc.List(ctx, playerRequester, c.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("List returned %d entries, want 1", len(entries))
	}
}

func TestJournalEntryService_List_OtherPlayerHidden(t *testing.T) {
	svc, characterSvc := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	other := fakeRequester{id: "other-1", gm: false}

	c, err := characterSvc.Create(ctx, other, "", "Beren")
	if err != nil {
		t.Fatalf("Create character: %v", err)
	}

	if _, err := svc.Create(ctx, other, c.ID, "Secret entry"); err != nil {
		t.Fatalf("Create entry: %v", err)
	}

	_, err = svc.List(ctx, playerRequester, c.ID)
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("List(other player's character) = %v, want ErrNotFound", err)
	}
}

func TestJournalEntryService_List_GMSeesAnyCharacter(t *testing.T) {
	svc, characterSvc := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	c, err := characterSvc.Create(ctx, playerRequester, "", "Aria")
	if err != nil {
		t.Fatalf("Create character: %v", err)
	}

	if _, err := svc.Create(ctx, playerRequester, c.ID, "Entry one"); err != nil {
		t.Fatalf("Create entry: %v", err)
	}

	entries, err := svc.List(ctx, gmRequester, c.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("List returned %d entries, want 1", len(entries))
	}
}

func TestJournalEntryService_Get_OtherPlayerHidden(t *testing.T) {
	svc, characterSvc := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	other := fakeRequester{id: "other-1", gm: false}

	c, err := characterSvc.Create(ctx, other, "", "Beren")
	if err != nil {
		t.Fatalf("Create character: %v", err)
	}

	e, err := svc.Create(ctx, other, c.ID, "Secret entry")
	if err != nil {
		t.Fatalf("Create entry: %v", err)
	}

	_, err = svc.Get(ctx, playerRequester, e.ID)
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Get(other player's entry) = %v, want ErrNotFound", err)
	}
}

func TestJournalEntryService_Update_OwnerCanEditContent(t *testing.T) {
	svc, characterSvc := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	c, err := characterSvc.Create(ctx, playerRequester, "", "Aria")
	if err != nil {
		t.Fatalf("Create character: %v", err)
	}

	e, err := svc.Create(ctx, playerRequester, c.ID, "Original")
	if err != nil {
		t.Fatalf("Create entry: %v", err)
	}

	updated, err := svc.Update(ctx, playerRequester, e.ID, "Revised")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Content != "Revised" {
		t.Fatalf("Content = %q, want %q", updated.Content, "Revised")
	}
	if updated.GameDay != e.GameDay {
		t.Fatalf("GameDay = %d, want unchanged %d", updated.GameDay, e.GameDay)
	}
}

func TestJournalEntryService_Update_OtherPlayerCannotEdit(t *testing.T) {
	svc, characterSvc := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	other := fakeRequester{id: "other-1", gm: false}

	c, err := characterSvc.Create(ctx, other, "", "Beren")
	if err != nil {
		t.Fatalf("Create character: %v", err)
	}

	e, err := svc.Create(ctx, other, c.ID, "Secret entry")
	if err != nil {
		t.Fatalf("Create entry: %v", err)
	}

	_, err = svc.Update(ctx, playerRequester, e.ID, "Hijacked")
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Update(other player's entry) = %v, want ErrNotFound", err)
	}
}

func TestJournalEntryService_Get_UnknownEntry(t *testing.T) {
	svc, _ := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	_, err := svc.Get(ctx, gmRequester, "does-not-exist")
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Get(unknown) = %v, want ErrNotFound", err)
	}
}
