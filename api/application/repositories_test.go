package application_test

import (
	"errors"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

type repositoryTestEnv struct {
	repos      *application.RepositoryService
	characters *application.CharacterService
	groups     *application.GroupService
}

func newTestRepositoryEnv(t *testing.T) repositoryTestEnv {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	if err != nil {
		t.Fatalf("persistence.New: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

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

	if err := env.repos.EnsureSystemRepositories(ctx); err != nil {
		t.Fatalf("EnsureSystemRepositories: %v", err)
	}
	if err := env.repos.EnsureSystemRepositories(ctx); err != nil {
		t.Fatalf("second EnsureSystemRepositories: %v", err)
	}

	repos, err := env.repos.List(ctx, gmRequester)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

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
	if general != 1 || template != 1 {
		t.Fatalf("general=%d template=%d, want exactly one of each", general, template)
	}
}

func TestRepositoryService_CharacterCreate_ProvisionsRepository(t *testing.T) {
	env := newTestRepositoryEnv(t)
	ctx := t.Context()

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	if err != nil {
		t.Fatalf("Create character: %v", err)
	}

	repos, err := env.repos.List(ctx, gmRequester)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	found := false
	for _, r := range repos {
		if r.Type == models.RepositoryTypeCharacter && r.CharacterID != nil && *r.CharacterID == character.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("no repository provisioned for character %s", character.ID)
	}
}

func TestRepositoryService_GroupCreate_ProvisionsRepository(t *testing.T) {
	env := newTestRepositoryEnv(t)
	ctx := t.Context()

	group, err := env.groups.Create(ctx, gmRequester, "Thieves Guild", models.GroupTypeOrganization, "")
	if err != nil {
		t.Fatalf("Create group: %v", err)
	}

	repos, err := env.repos.List(ctx, gmRequester)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	found := false
	for _, r := range repos {
		if r.Type == models.RepositoryTypeGroup && r.GroupID != nil && *r.GroupID == group.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("no repository provisioned for group %s", group.ID)
	}
}

func TestRepositoryService_Get_GeneralAndTemplateVisibleToEveryone(t *testing.T) {
	env := newTestRepositoryEnv(t)
	ctx := t.Context()

	if err := env.repos.EnsureSystemRepositories(ctx); err != nil {
		t.Fatalf("EnsureSystemRepositories: %v", err)
	}

	repos, err := env.repos.List(ctx, gmRequester)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	for _, r := range repos {
		if _, err := env.repos.Get(ctx, playerRequester, r.ID); err != nil {
			t.Fatalf("Get(%s) as player = %v, want nil", r.Type, err)
		}
	}
}

func TestRepositoryService_Get_CharacterRepositoryOwnerOnly(t *testing.T) {
	env := newTestRepositoryEnv(t)
	ctx := t.Context()

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	if err != nil {
		t.Fatalf("Create character: %v", err)
	}

	repos, err := env.repos.List(ctx, gmRequester)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var repoID string
	for _, r := range repos {
		if r.Type == models.RepositoryTypeCharacter && r.CharacterID != nil && *r.CharacterID == character.ID {
			repoID = r.ID
		}
	}
	if repoID == "" {
		t.Fatalf("character repository not found")
	}

	if _, err := env.repos.Get(ctx, playerRequester, repoID); err != nil {
		t.Fatalf("Get as owner: %v", err)
	}
	if _, err := env.repos.Get(ctx, gmRequester, repoID); err != nil {
		t.Fatalf("Get as GM: %v", err)
	}

	// A different player must not learn the repository exists.
	_, err = env.repos.Get(ctx, otherPlayer, repoID)
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Get as foreign player = %v, want ErrNotFound", err)
	}
}

func TestRepositoryService_Get_GroupRepositoryMembersOnly(t *testing.T) {
	env := newTestRepositoryEnv(t)
	ctx := t.Context()

	group, err := env.groups.Create(ctx, gmRequester, "Thieves Guild", models.GroupTypeOrganization, "")
	if err != nil {
		t.Fatalf("Create group: %v", err)
	}
	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	if err != nil {
		t.Fatalf("Create character: %v", err)
	}

	repos, err := env.repos.List(ctx, gmRequester)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var repoID string
	for _, r := range repos {
		if r.Type == models.RepositoryTypeGroup && r.GroupID != nil && *r.GroupID == group.ID {
			repoID = r.ID
		}
	}
	if repoID == "" {
		t.Fatalf("group repository not found")
	}

	// Not a member yet: hidden.
	_, err = env.repos.Get(ctx, playerRequester, repoID)
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Get before joining = %v, want ErrNotFound", err)
	}

	if err := env.groups.Join(ctx, playerRequester, group.ID, character.ID); err != nil {
		t.Fatalf("Join: %v", err)
	}

	if _, err := env.repos.Get(ctx, playerRequester, repoID); err != nil {
		t.Fatalf("Get after joining: %v", err)
	}

	// A non-member player still can't see it.
	_, err = env.repos.Get(ctx, otherPlayer, repoID)
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Get as non-member = %v, want ErrNotFound", err)
	}
}

func TestRepositoryService_List_PlayerSeesOnlyOwnAndSystemRepositories(t *testing.T) {
	env := newTestRepositoryEnv(t)
	ctx := t.Context()

	if err := env.repos.EnsureSystemRepositories(ctx); err != nil {
		t.Fatalf("EnsureSystemRepositories: %v", err)
	}
	if _, err := env.characters.Create(ctx, playerRequester, "", "Aria"); err != nil {
		t.Fatalf("Create character: %v", err)
	}
	if _, err := env.characters.Create(ctx, otherPlayer, "", "Beren"); err != nil {
		t.Fatalf("Create other character: %v", err)
	}

	repos, err := env.repos.List(ctx, playerRequester)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	// general + template + this player's own character repository.
	if len(repos) != 3 {
		t.Fatalf("List returned %d repositories, want 3", len(repos))
	}
	for _, r := range repos {
		if r.Type == models.RepositoryTypeCharacter && (r.CharacterID == nil || *r.CharacterID == "") {
			t.Fatalf("character repository missing character_id")
		}
	}
}
