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

type repositoryTestEnv struct {
	repos      *application.RepositoryService
	characters *application.CharacterService
	groups     *application.GroupService
}

func newTestRepositoryEnv(t *testing.T) repositoryTestEnv {
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

	return repositoryTestEnv{repos: repoSvc, characters: charSvc, groups: groupSvc}
}

func TestRepositoryService_EnsureSystemRepositories_IsIdempotent(t *testing.T) {
	env := newTestRepositoryEnv(t)
	ctx := t.Context()

	err := env.repos.EnsureSystemRepositories(ctx)
	require.NoError(t, err)
	err = env.repos.EnsureSystemRepositories(ctx)
	require.NoError(t, err)

	repos, err := env.repos.List(ctx, gmRequester)
	require.NoError(t, err)

	var general, template int
	for _, r := range repos {
		switch r.Type {
		case models.RepositoryTypeGeneral:
			general++
		case models.RepositoryTypeTemplate:
			template++
		case models.RepositoryTypeGroup, models.RepositoryTypeCharacter:
			// not relevant to this assertion
		}
	}
	assert.Equal(t, 1, general, "want exactly one general repository")
	assert.Equal(t, 1, template, "want exactly one template repository")
}

func TestRepositoryService_CharacterCreate_ProvisionsRepository(t *testing.T) {
	env := newTestRepositoryEnv(t)
	ctx := t.Context()

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	repos, err := env.repos.List(ctx, gmRequester)
	require.NoError(t, err)

	found := false
	for _, r := range repos {
		if r.Type == models.RepositoryTypeCharacter && r.CharacterID != nil && *r.CharacterID == character.ID {
			found = true
		}
	}
	assert.True(t, found, "no repository provisioned for character %s", character.ID)
}

func TestRepositoryService_GroupCreate_ProvisionsRepository(t *testing.T) {
	env := newTestRepositoryEnv(t)
	ctx := t.Context()

	group, err := env.groups.Create(ctx, gmRequester, "Thieves Guild", models.GroupTypeOrganization, "")
	require.NoError(t, err)

	repos, err := env.repos.List(ctx, gmRequester)
	require.NoError(t, err)

	found := false
	for _, r := range repos {
		if r.Type == models.RepositoryTypeGroup && r.GroupID != nil && *r.GroupID == group.ID {
			found = true
		}
	}
	assert.True(t, found, "no repository provisioned for group %s", group.ID)
}

func TestRepositoryService_Get_GeneralAndTemplateVisibleToEveryone(t *testing.T) {
	env := newTestRepositoryEnv(t)
	ctx := t.Context()

	err := env.repos.EnsureSystemRepositories(ctx)
	require.NoError(t, err)

	repos, err := env.repos.List(ctx, gmRequester)
	require.NoError(t, err)

	for _, r := range repos {
		_, err := env.repos.Get(ctx, playerRequester, r.ID)
		require.NoError(t, err, "Get(%s) as player", r.Type)
	}
}

func TestRepositoryService_Get_CharacterRepositoryOwnerOnly(t *testing.T) {
	env := newTestRepositoryEnv(t)
	ctx := t.Context()

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	repos, err := env.repos.List(ctx, gmRequester)
	require.NoError(t, err)

	var repoID string
	for _, r := range repos {
		if r.Type == models.RepositoryTypeCharacter && r.CharacterID != nil && *r.CharacterID == character.ID {
			repoID = r.ID
		}
	}
	require.NotEmpty(t, repoID, "character repository not found")

	_, err = env.repos.Get(ctx, playerRequester, repoID)
	require.NoError(t, err, "Get as owner")
	_, err = env.repos.Get(ctx, gmRequester, repoID)
	require.NoError(t, err, "Get as GM")

	// A different player must not learn the repository exists.
	_, err = env.repos.Get(ctx, otherPlayer, repoID)
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestRepositoryService_Get_GroupRepositoryMembersOnly(t *testing.T) {
	env := newTestRepositoryEnv(t)
	ctx := t.Context()

	group, err := env.groups.Create(ctx, gmRequester, "Thieves Guild", models.GroupTypeOrganization, "")
	require.NoError(t, err)
	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	repos, err := env.repos.List(ctx, gmRequester)
	require.NoError(t, err)

	var repoID string
	for _, r := range repos {
		if r.Type == models.RepositoryTypeGroup && r.GroupID != nil && *r.GroupID == group.ID {
			repoID = r.ID
		}
	}
	require.NotEmpty(t, repoID, "group repository not found")

	// Not a member yet: hidden.
	_, err = env.repos.Get(ctx, playerRequester, repoID)
	require.ErrorIs(t, err, application.ErrNotFound)

	err = env.groups.Join(ctx, playerRequester, group.ID, character.ID)
	require.NoError(t, err)

	_, err = env.repos.Get(ctx, playerRequester, repoID)
	require.NoError(t, err, "Get after joining")

	// A non-member player still can't see it.
	_, err = env.repos.Get(ctx, otherPlayer, repoID)
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestRepositoryService_List_PlayerSeesOnlyOwnAndSystemRepositories(t *testing.T) {
	env := newTestRepositoryEnv(t)
	ctx := t.Context()

	err := env.repos.EnsureSystemRepositories(ctx)
	require.NoError(t, err)
	_, err = env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)
	_, err = env.characters.Create(ctx, otherPlayer, "", "Beren")
	require.NoError(t, err)

	repos, err := env.repos.List(ctx, playerRequester)
	require.NoError(t, err)

	// general + template + this player's own character repository.
	require.Len(t, repos, 3)
	for _, r := range repos {
		if r.Type == models.RepositoryTypeCharacter {
			assert.NotEmpty(t, r.CharacterID, "character repository missing character_id")
			if r.CharacterID != nil {
				assert.NotEmpty(t, *r.CharacterID, "character repository missing character_id")
			}
		}
	}
}
