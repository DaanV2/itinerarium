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

type vaultImportTestEnv struct {
	documentTestEnv
	vault *application.VaultImportService
}

func newVaultImportTestEnv(t *testing.T) vaultImportTestEnv {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err)
	err = db.Migrate()
	require.NoError(t, err)

	knowledgeRepo := repositories.NewKnowledgeRepositories(db)
	characterRepo := repositories.NewCharacters(db)
	groupRepo := repositories.NewGroups(db)

	charSvc := application.NewCharacterService(characterRepo, repositories.NewUsers(db), knowledgeRepo)
	groupSvc := application.NewGroupService(groupRepo, charSvc, knowledgeRepo)
	repoSvc := application.NewRepositoryService(knowledgeRepo, groupRepo, characterRepo)
	docSvc := application.NewDocumentService(
		repositories.NewDocuments(db), repoSvc, characterRepo, groupRepo, repositories.NewDocumentShares(db),
	)
	vaultSvc := application.NewVaultImportService(docSvc, repoSvc, knowledgeRepo, groupRepo, characterRepo)

	err = repoSvc.EnsureSystemRepositories(t.Context())
	require.NoError(t, err)

	return vaultImportTestEnv{
		documentTestEnv: documentTestEnv{docs: docSvc, repos: repoSvc, characters: charSvc, groups: groupSvc},
		vault:           vaultSvc,
	}
}

func TestVaultImportService_Import_MapsFoldersAndFrontmatter(t *testing.T) {
	env := newVaultImportTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	markdown := "---\ntitle: The Prophecy\ntags: [lore, dragons]\ngame_day: 4\n---\nA dragon will rise."

	results, err := env.vault.Import(ctx, gmRequester, general.ID, []application.ImportFileInput{
		{Path: "lore/prophecies/first.md", Markdown: markdown},
		{Path: "notes/plain.md", Markdown: "No frontmatter here."},
	})
	require.NoError(t, err)
	require.Len(t, results, 2)

	require.Equal(t, application.ImportStatusImported, results[0].Status)
	assert.Equal(t, general.ID, results[0].RepositoryID)

	view, err := env.docs.Get(ctx, gmRequester, results[0].DocumentID)
	require.NoError(t, err)
	assert.Equal(t, "lore/prophecies/first", view.Document.Path, "folders map to the path, .md is dropped")
	assert.Equal(t, "The Prophecy", view.Document.Title)
	assert.Equal(t, []string{"lore", "dragons"}, view.Document.Tags)
	assert.Equal(t, 4, view.Document.SharedOnGameDay)
	require.Len(t, view.Document.Sections, 1)
	assert.Equal(t, "A dragon will rise.", view.Document.Sections[0].Content)

	require.Equal(t, application.ImportStatusImported, results[1].Status)

	view, err = env.docs.Get(ctx, gmRequester, results[1].DocumentID)
	require.NoError(t, err)
	assert.Equal(t, "plain", view.Document.Title, "title falls back to the file name")
}

func TestVaultImportService_Import_FrontmatterRepositoryTargets(t *testing.T) {
	env := newVaultImportTestEnv(t)
	ctx := t.Context()

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	group, err := env.groups.Create(ctx, gmRequester, "Thieves Guild", models.GroupTypeOrganization, "")
	require.NoError(t, err)

	general := env.findRepository(t, models.RepositoryTypeGeneral, "")
	templateRepo := env.findRepository(t, models.RepositoryTypeTemplate, "")
	groupRepo := env.findRepository(t, models.RepositoryTypeGroup, group.ID)
	characterRepo := env.findRepository(t, models.RepositoryTypeCharacter, character.ID)

	results, err := env.vault.Import(ctx, gmRequester, general.ID, []application.ImportFileInput{
		{Path: "a.md", Markdown: "---\nrepository: template\n---\nBody."},
		{Path: "b.md", Markdown: "---\nrepository: Thieves Guild\n---\nBody."},
		{Path: "c.md", Markdown: "---\nrepository: Aria\n---\nBody."},
		{Path: "d.md", Markdown: "Body without a repository."},
	})
	require.NoError(t, err)
	require.Len(t, results, 4)

	for _, r := range results {
		require.Equal(t, application.ImportStatusImported, r.Status, "file %s: %s", r.Path, r.Error)
	}

	assert.Equal(t, templateRepo.ID, results[0].RepositoryID)
	assert.Equal(t, groupRepo.ID, results[1].RepositoryID)
	assert.Equal(t, characterRepo.ID, results[2].RepositoryID)
	assert.Equal(t, general.ID, results[3].RepositoryID, "no frontmatter target falls back to the default")
}

func TestVaultImportService_Import_CollisionWarnsThenRenameOrContinue(t *testing.T) {
	env := newVaultImportTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	results, err := env.vault.Import(ctx, gmRequester, general.ID, []application.ImportFileInput{
		{Path: "lore/origins.md", Markdown: "First version."},
	})
	require.NoError(t, err)
	require.Equal(t, application.ImportStatusImported, results[0].Status)

	// Same path again: a warning, nothing imported.
	results, err = env.vault.Import(ctx, gmRequester, general.ID, []application.ImportFileInput{
		{Path: "lore/origins.md", Markdown: "Second version."},
	})
	require.NoError(t, err)
	require.Equal(t, application.ImportStatusCollision, results[0].Status)
	assert.Empty(t, results[0].DocumentID)

	// Rename resolves it.
	results, err = env.vault.Import(ctx, gmRequester, general.ID, []application.ImportFileInput{
		{Path: "lore/origins-2.md", Markdown: "Second version."},
	})
	require.NoError(t, err)
	assert.Equal(t, application.ImportStatusImported, results[0].Status)

	// So does continuing anyway.
	results, err = env.vault.Import(ctx, gmRequester, general.ID, []application.ImportFileInput{
		{Path: "lore/origins.md", Markdown: "Second version.", AllowCollision: true},
	})
	require.NoError(t, err)
	assert.Equal(t, application.ImportStatusImported, results[0].Status)
}

func TestVaultImportService_Import_InaccessibleDefaultRepositoryIsNotFound(t *testing.T) {
	env := newVaultImportTestEnv(t)
	ctx := t.Context()

	_, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	other := fakeRequester{id: "player-2", gm: false}
	otherCharacter, err := env.characters.Create(ctx, other, "", "Beren")
	require.NoError(t, err)

	otherRepo := env.findRepository(t, models.RepositoryTypeCharacter, otherCharacter.ID)

	_, err = env.vault.Import(ctx, playerRequester, otherRepo.ID, []application.ImportFileInput{
		{Path: "sneaky.md", Markdown: "Body."},
	})
	require.ErrorIs(t, err, application.ErrNotFound, "never 403 — existence must not leak")
}

func TestVaultImportService_Import_InaccessibleFrontmatterTargetReadsAsUnknown(t *testing.T) {
	env := newVaultImportTestEnv(t)
	ctx := t.Context()

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)
	ownRepo := env.findRepository(t, models.RepositoryTypeCharacter, character.ID)

	other := fakeRequester{id: "player-2", gm: false}
	_, err = env.characters.Create(ctx, other, "", "Beren")
	require.NoError(t, err)

	results, err := env.vault.Import(ctx, playerRequester, ownRepo.ID, []application.ImportFileInput{
		{Path: "a.md", Markdown: "---\nrepository: Beren\n---\nProbe an existing character."},
		{Path: "b.md", Markdown: "---\nrepository: Nobody\n---\nProbe a nonexistent one."},
	})
	require.NoError(t, err)
	require.Len(t, results, 2)

	require.Equal(t, application.ImportStatusError, results[0].Status)
	require.Equal(t, application.ImportStatusError, results[1].Status)
	assert.Equal(t,
		`repository "Beren" not found`, results[0].Error,
		"an existing but inaccessible repository must read exactly like a nonexistent one",
	)
	assert.Equal(t, `repository "Nobody" not found`, results[1].Error)
}

func TestVaultImportService_Import_PlayerImportsIntoOwnRepository(t *testing.T) {
	env := newVaultImportTestEnv(t)
	ctx := t.Context()

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)
	ownRepo := env.findRepository(t, models.RepositoryTypeCharacter, character.ID)

	results, err := env.vault.Import(ctx, playerRequester, ownRepo.ID, []application.ImportFileInput{
		{Path: "diary/day-one.md", Markdown: "My first day."},
	})
	require.NoError(t, err)
	require.Equal(t, application.ImportStatusImported, results[0].Status)
	assert.Equal(t, ownRepo.ID, results[0].RepositoryID)
}

func TestVaultImportService_Import_RequestValidation(t *testing.T) {
	env := newVaultImportTestEnv(t)
	ctx := t.Context()

	_, err := env.vault.Import(ctx, gmRequester, "", nil)
	require.ErrorIs(t, err, application.ErrInvalidImport)

	results, err := env.vault.Import(ctx, gmRequester, "", []application.ImportFileInput{
		{Path: "orphan.md", Markdown: "No default, no frontmatter."},
	})
	require.NoError(t, err)
	require.Equal(t, application.ImportStatusError, results[0].Status)
	assert.Contains(t, results[0].Error, "no target repository")
}
