package application_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestJournalEntriesEnv(
	t *testing.T,
) (*application.JournalEntryService, *application.CharacterService, *application.DocumentService) {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err)
	err = db.Migrate()
	require.NoError(t, err)

	users := repositories.NewUsers(db)
	characters := repositories.NewCharacters(db)
	entries := repositories.NewJournalEntries(db)
	knowledgeRepos := repositories.NewKnowledgeRepositories(db)
	groups := repositories.NewGroups(db)

	characterSvc := application.NewCharacterService(characters, users, knowledgeRepos)
	repositoryService := application.NewRepositoryService(knowledgeRepos, groups, characters)
	documentSvc := application.NewDocumentService(
		repositories.NewDocuments(db), repositoryService, characters, groups, repositories.NewDocumentShares(db),
	)

	return application.NewJournalEntryService(entries, characters, documentSvc, knowledgeRepos), characterSvc, documentSvc
}

func TestJournalEntryService_Create_StampsCurrentGameDay(t *testing.T) {
	svc, characterSvc, _ := newTestJournalEntriesEnv(t)
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
	svc, characterSvc, _ := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	c, err := characterSvc.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	_, err = svc.Create(ctx, playerRequester, c.ID, "")
	require.ErrorIs(t, err, application.ErrInvalidContent)
}

func TestJournalEntryService_Create_OtherPlayerCannotWriteForForeignCharacter(t *testing.T) {
	svc, characterSvc, _ := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	other := fakeRequester{id: "other-1", gm: false}

	c, err := characterSvc.Create(ctx, other, "", "Beren")
	require.NoError(t, err)

	_, err = svc.Create(ctx, playerRequester, c.ID, "Not mine to write")
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestJournalEntryService_List_OwnerSeesOwnEntries(t *testing.T) {
	svc, characterSvc, _ := newTestJournalEntriesEnv(t)
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
	svc, characterSvc, _ := newTestJournalEntriesEnv(t)
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
	svc, characterSvc, _ := newTestJournalEntriesEnv(t)
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
	svc, characterSvc, _ := newTestJournalEntriesEnv(t)
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
	svc, characterSvc, _ := newTestJournalEntriesEnv(t)
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
	svc, characterSvc, _ := newTestJournalEntriesEnv(t)
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
	svc, _, _ := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	_, err := svc.Get(ctx, gmRequester, "does-not-exist")
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestJournalEntryService_Convert_CopiesIntoCharacterRepository(t *testing.T) {
	svc, characterSvc, documentSvc := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	c, err := characterSvc.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	e, err := svc.Create(ctx, playerRequester, c.ID, "Dear diary, today I met a dragon.")
	require.NoError(t, err)

	view, err := svc.Convert(ctx, playerRequester, e.ID)
	require.NoError(t, err)
	assert.Equal(t, "Dear diary, today I met a dragon.", view.Document.Sections[0].Content)
	assert.Equal(t, "Dear diary, today I met a dragon.", view.Document.Title)

	// The journal entry itself is untouched.
	untouched, err := svc.Get(ctx, playerRequester, e.ID)
	require.NoError(t, err)
	assert.Equal(t, "Dear diary, today I met a dragon.", untouched.Content)

	// The document landed in the character's own repository, still visible to
	// the owner.
	got, err := documentSvc.Get(ctx, playerRequester, view.Document.ID)
	require.NoError(t, err)
	assert.Equal(t, view.Document.ID, got.Document.ID)
}

func TestJournalEntryService_Convert_OtherPlayerCannotConvert(t *testing.T) {
	svc, characterSvc, _ := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	other := fakeRequester{id: "other-1", gm: false}

	c, err := characterSvc.Create(ctx, other, "", "Beren")
	require.NoError(t, err)

	e, err := svc.Create(ctx, other, c.ID, "Secret entry")
	require.NoError(t, err)

	_, err = svc.Convert(ctx, playerRequester, e.ID)
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestJournalEntryService_Convert_GMCanConvertForAnyCharacter(t *testing.T) {
	svc, characterSvc, _ := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	c, err := characterSvc.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	e, err := svc.Create(ctx, playerRequester, c.ID, "Entry")
	require.NoError(t, err)

	view, err := svc.Convert(ctx, gmRequester, e.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, view.Document.ID)
}

func TestJournalEntryService_Convert_OtherPlayerCannotSeeConvertedDocument(t *testing.T) {
	svc, characterSvc, documentSvc := newTestJournalEntriesEnv(t)
	ctx := t.Context()

	c, err := characterSvc.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	e, err := svc.Create(ctx, playerRequester, c.ID, "Private thoughts")
	require.NoError(t, err)

	view, err := svc.Convert(ctx, playerRequester, e.ID)
	require.NoError(t, err)

	other := fakeRequester{id: "other-1", gm: false}
	_, err = documentSvc.Get(ctx, other, view.Document.ID)
	require.ErrorIs(t, err, application.ErrNotFound)
}
