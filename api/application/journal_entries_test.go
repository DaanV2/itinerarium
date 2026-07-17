package application_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestJournalEntriesEnv(t *testing.T) (*application.JournalEntryService, *application.CharacterService) {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err)
	err = db.Migrate()
	require.NoError(t, err)

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
	require.NoError(t, err)

	day := 3

	_, err = characterSvc.Update(ctx, gmRequester, c.ID, nil, &day)
	require.NoError(t, err)

	e, err := svc.Create(ctx, playerRequester, c.ID, "Dear diary...")
	require.NoError(t, err)
	assert.Equal(t, day, e.GameDay)
	assert.Equal(t, "Dear diary...", e.Content)
}

func TestJournalEntryService_Create_RejectsEmptyContent(t *testing.T) {
	svc, characterSvc := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	c, err := characterSvc.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	_, err = svc.Create(ctx, playerRequester, c.ID, "")
	require.ErrorIs(t, err, application.ErrInvalidContent)
}

func TestJournalEntryService_Create_OtherPlayerCannotWriteForForeignCharacter(t *testing.T) {
	svc, characterSvc := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	other := fakeRequester{id: "other-1", gm: false}

	c, err := characterSvc.Create(ctx, other, "", "Beren")
	require.NoError(t, err)

	_, err = svc.Create(ctx, playerRequester, c.ID, "Not mine to write")
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestJournalEntryService_List_OwnerSeesOwnEntries(t *testing.T) {
	svc, characterSvc := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	c, err := characterSvc.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	_, err = svc.Create(ctx, playerRequester, c.ID, "Entry one")
	require.NoError(t, err)

	entries, err := svc.List(ctx, playerRequester, c.ID)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestJournalEntryService_List_OtherPlayerHidden(t *testing.T) {
	svc, characterSvc := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	other := fakeRequester{id: "other-1", gm: false}

	c, err := characterSvc.Create(ctx, other, "", "Beren")
	require.NoError(t, err)

	_, err = svc.Create(ctx, other, c.ID, "Secret entry")
	require.NoError(t, err)

	_, err = svc.List(ctx, playerRequester, c.ID)
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestJournalEntryService_List_GMSeesAnyCharacter(t *testing.T) {
	svc, characterSvc := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	c, err := characterSvc.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	_, err = svc.Create(ctx, playerRequester, c.ID, "Entry one")
	require.NoError(t, err)

	entries, err := svc.List(ctx, gmRequester, c.ID)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestJournalEntryService_Get_OtherPlayerHidden(t *testing.T) {
	svc, characterSvc := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	other := fakeRequester{id: "other-1", gm: false}

	c, err := characterSvc.Create(ctx, other, "", "Beren")
	require.NoError(t, err)

	e, err := svc.Create(ctx, other, c.ID, "Secret entry")
	require.NoError(t, err)

	_, err = svc.Get(ctx, playerRequester, e.ID)
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestJournalEntryService_Update_OwnerCanEditContent(t *testing.T) {
	svc, characterSvc := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	c, err := characterSvc.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	e, err := svc.Create(ctx, playerRequester, c.ID, "Original")
	require.NoError(t, err)

	updated, err := svc.Update(ctx, playerRequester, e.ID, "Revised")
	require.NoError(t, err)
	assert.Equal(t, "Revised", updated.Content)
	assert.Equal(t, e.GameDay, updated.GameDay, "GameDay should remain unchanged")
}

func TestJournalEntryService_Update_OtherPlayerCannotEdit(t *testing.T) {
	svc, characterSvc := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	other := fakeRequester{id: "other-1", gm: false}

	c, err := characterSvc.Create(ctx, other, "", "Beren")
	require.NoError(t, err)

	e, err := svc.Create(ctx, other, c.ID, "Secret entry")
	require.NoError(t, err)

	_, err = svc.Update(ctx, playerRequester, e.ID, "Hijacked")
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestJournalEntryService_Get_UnknownEntry(t *testing.T) {
	svc, _ := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	_, err := svc.Get(ctx, gmRequester, "does-not-exist")
	require.ErrorIs(t, err, application.ErrNotFound)
}
