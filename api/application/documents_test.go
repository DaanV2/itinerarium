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

type documentTestEnv struct {
	docs       *application.DocumentService
	repos      *application.RepositoryService
	characters *application.CharacterService
	groups     *application.GroupService
}

func newDocumentTestEnv(t *testing.T) documentTestEnv {
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

	err = repoSvc.EnsureSystemRepositories(t.Context())
	require.NoError(t, err)

	return documentTestEnv{docs: docSvc, repos: repoSvc, characters: charSvc, groups: groupSvc}
}

// findRepository returns the one repository matching the type and (for group/
// character repositories) the owner ID.
func (e documentTestEnv) findRepository(
	t *testing.T, repoType models.RepositoryType, ownerID string,
) *models.Repository {
	t.Helper()

	repos, err := e.repos.List(t.Context(), gmRequester)
	require.NoError(t, err)

	for i := range repos {
		r := &repos[i]
		if r.Type != repoType {
			continue
		}

		switch repoType {
		case models.RepositoryTypeGroup:
			if r.GroupID != nil && *r.GroupID == ownerID {
				return r
			}
		case models.RepositoryTypeCharacter:
			if r.CharacterID != nil && *r.CharacterID == ownerID {
				return r
			}
		case models.RepositoryTypeGeneral, models.RepositoryTypeTemplate:
			return r
		}
	}

	require.Failf(t, "repository not found", "no %s repository found for owner %q", repoType, ownerID)

	return nil
}

func (e documentTestEnv) setGameDay(t *testing.T, characterID string, day int) {
	t.Helper()

	_, err := e.characters.Update(t.Context(), gmRequester, characterID, nil, &day)
	require.NoError(t, err)
}

func mustCreateDocument(
	t *testing.T, env documentTestEnv,
	requester application.Requester, repoID string, input *application.CreateDocumentInput,
) *application.DocumentView {
	t.Helper()

	view, err := env.docs.Create(t.Context(), requester, repoID, input)
	require.NoError(t, err)

	return view
}

func TestDocumentService_Delete_GMOnly(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	view := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path:     "lore/doomed",
		Sections: []application.DocumentSectionInput{{Content: "Soon gone."}},
	})

	err := env.docs.Delete(ctx, playerRequester, view.Document.ID)
	require.ErrorIs(t, err, application.ErrForbidden)

	err = env.docs.Delete(ctx, gmRequester, view.Document.ID)
	require.NoError(t, err)
	_, err = env.docs.Get(ctx, gmRequester, view.Document.ID)
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestDocumentService_CreateAndGet_TitleFallsBackToFileName(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	view := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path:     "factions//thieves-guild",
		Sections: []application.DocumentSectionInput{{Content: "The guild runs the docks."}},
	})

	assert.Equal(t, "factions/thieves-guild", view.Document.Path)
	assert.Equal(t, "thieves-guild", view.Document.Title)

	got, err := env.docs.Get(ctx, gmRequester, view.Document.ID)
	require.NoError(t, err)
	if assert.Len(t, got.Document.Sections, 1, "want the created section back") {
		assert.Equal(t, "The guild runs the docks.", got.Document.Sections[0].Content)
	}
}

func TestDocumentService_GameDayGatesVisibility_IncludingRewind(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	doc := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path:            "reveals/the-betrayal",
		SharedOnGameDay: new(5),
		Sections:        []application.DocumentSectionInput{{Content: "He was the traitor all along."}},
	})

	// Day 0 < 5: absent from lists AND a direct Get is a 404, never a 403.
	_, err = env.docs.Get(ctx, playerRequester, doc.Document.ID)
	require.ErrorIs(t, err, application.ErrNotFound)

	listed, err := env.docs.ListByRepository(ctx, playerRequester, general.ID)
	require.NoError(t, err)
	assert.Empty(t, listed)

	// Reaching day 5 reveals it.
	env.setGameDay(t, character.ID, 5)

	_, err = env.docs.Get(ctx, playerRequester, doc.Document.ID)
	require.NoError(t, err)

	// Rewinding hides it again.
	env.setGameDay(t, character.ID, 4)

	_, err = env.docs.Get(ctx, playerRequester, doc.Document.ID)
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestDocumentService_GroupRepository_HiddenFromNonMembers(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()

	member, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	outsider := fakeRequester{id: "outsider-1", gm: false}
	_, err = env.characters.Create(ctx, outsider, "", "Beren")
	require.NoError(t, err)

	group, err := env.groups.Create(ctx, gmRequester, "Thieves Guild", models.GroupTypeOrganization, "")
	require.NoError(t, err)
	err = env.groups.Join(ctx, gmRequester, group.ID, member.ID)
	require.NoError(t, err)

	groupRepo := env.findRepository(t, models.RepositoryTypeGroup, group.ID)
	doc := mustCreateDocument(t, env, gmRequester, groupRepo.ID, &application.CreateDocumentInput{
		Path:     "plans/heist",
		Sections: []application.DocumentSectionInput{{Content: "We strike at midnight."}},
	})

	_, err = env.docs.Get(ctx, playerRequester, doc.Document.ID)
	require.NoError(t, err)

	_, err = env.docs.Get(ctx, outsider, doc.Document.ID)
	require.ErrorIs(t, err, application.ErrNotFound)
	_, err = env.docs.ListByRepository(ctx, outsider, groupRepo.ID)
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestDocumentService_CharacterRepository_HiddenFromOtherPlayers(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	other := fakeRequester{id: "other-1", gm: false}
	_, err = env.characters.Create(ctx, other, "", "Beren")
	require.NoError(t, err)

	charRepo := env.findRepository(t, models.RepositoryTypeCharacter, character.ID)
	doc := mustCreateDocument(t, env, playerRequester, charRepo.ID, &application.CreateDocumentInput{
		Path:     "notes/suspicions",
		Sections: []application.DocumentSectionInput{{Content: "I do not trust the duke."}},
	})

	_, err = env.docs.Get(ctx, playerRequester, doc.Document.ID)
	require.NoError(t, err)
	_, err = env.docs.Get(ctx, gmRequester, doc.Document.ID)
	require.NoError(t, err)

	_, err = env.docs.Get(ctx, other, doc.Document.ID)
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestDocumentService_GMOnlySections_StrippedForPlayers(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	_, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	doc := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path: "npcs/duke",
		Sections: []application.DocumentSectionInput{
			{Content: "The duke rules the city."},
			{Content: "He is secretly a vampire.", GMOnly: true},
		},
	})

	playerView, err := env.docs.Get(ctx, playerRequester, doc.Document.ID)
	require.NoError(t, err)
	if assert.Len(t, playerView.Document.Sections, 1) {
		for _, sec := range playerView.Document.Sections {
			assert.False(t, sec.GMOnly, "GM-only content leaked to player: %+v", sec)
			assert.NotEqual(t, "He is secretly a vampire.", sec.Content, "GM-only content leaked to player: %+v", sec)
		}
	}

	gmView, err := env.docs.Get(ctx, gmRequester, doc.Document.ID)
	require.NoError(t, err)
	assert.Len(t, gmView.Document.Sections, 2)
}

func TestDocumentService_PlayerCannotCreateGMOnlySection(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	_, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	_, err = env.docs.Create(ctx, playerRequester, general.ID, &application.CreateDocumentInput{
		Path:     "npcs/duke",
		Sections: []application.DocumentSectionInput{{Content: "secret", GMOnly: true}},
	})
	require.ErrorIs(t, err, application.ErrForbidden)
}

func TestDocumentService_PathCollision_WarnsThenAllows(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path: "lore/creation",
	})

	_, err := env.docs.Create(ctx, gmRequester, general.ID, &application.CreateDocumentInput{Path: "lore/creation"})
	require.ErrorIs(t, err, application.ErrPathCollision)

	_, err = env.docs.Create(ctx, gmRequester, general.ID, &application.CreateDocumentInput{
		Path: "lore/creation", AllowCollision: true,
	})
	require.NoError(t, err)
}

func TestDocumentService_PathCollision_OnMove(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{Path: "lore/creation"})
	doc := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{Path: "lore/draft"})

	update := application.UpdateDocumentInput{Path: "lore/creation", Title: doc.Document.Title}
	_, err := env.docs.Update(ctx, gmRequester, doc.Document.ID, &update)
	require.ErrorIs(t, err, application.ErrPathCollision)

	update.AllowCollision = true
	_, err = env.docs.Update(ctx, gmRequester, doc.Document.ID, &update)
	require.NoError(t, err)
}

func TestDocumentService_ConcurrentEdit_WarnsThenForces(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	created := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path:     "lore/creation",
		Sections: []application.DocumentSectionInput{{Content: "v1"}},
	})

	loaded, err := env.docs.Get(ctx, gmRequester, created.Document.ID)
	require.NoError(t, err)
	staleVersion := loaded.Document.Version

	// First save with the loaded version succeeds.
	first := application.UpdateDocumentInput{
		Path:            loaded.Document.Path,
		Title:           loaded.Document.Title,
		Sections:        []application.DocumentSectionInput{{ID: loaded.Document.Sections[0].ID, Content: "v2"}},
		ExpectedVersion: &staleVersion,
	}
	_, err = env.docs.Update(ctx, gmRequester, created.Document.ID, &first)
	require.NoError(t, err)

	// A second save still holding the old version warns.
	second := first
	second.Sections = []application.DocumentSectionInput{{ID: loaded.Document.Sections[0].ID, Content: "v3"}}
	_, err = env.docs.Update(ctx, gmRequester, created.Document.ID, &second)
	require.ErrorIs(t, err, application.ErrConcurrentEdit)

	// Forcing overwrites anyway.
	second.Force = true
	updated, err := env.docs.Update(ctx, gmRequester, created.Document.ID, &second)
	require.NoError(t, err)
	assert.Equal(t, "v3", updated.Document.Sections[0].Content)
}

func TestDocumentService_PlayerEditOnAllGMOnlyDocument_AppendsVisibleSection(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	_, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	doc := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path:     "npcs/duke",
		Sections: []application.DocumentSectionInput{{Content: "He is secretly a vampire.", GMOnly: true}},
	})

	// The player opens an apparently empty document and writes into it.
	playerView, err := env.docs.Get(ctx, playerRequester, doc.Document.ID)
	require.NoError(t, err)
	assert.Empty(t, playerView.Document.Sections)

	_, err = env.docs.Update(ctx, playerRequester, doc.Document.ID, &application.UpdateDocumentInput{
		Path:     playerView.Document.Path,
		Title:    playerView.Document.Title,
		Sections: []application.DocumentSectionInput{{Content: "The duke seems friendly."}},
	})
	require.NoError(t, err)

	gmView, err := env.docs.Get(ctx, gmRequester, doc.Document.ID)
	require.NoError(t, err)
	if assert.Len(t, gmView.Document.Sections, 2, "want GM section + new player section") {
		assert.True(t, gmView.Document.Sections[0].GMOnly)
		assert.Equal(t, "He is secretly a vampire.", gmView.Document.Sections[0].Content,
			"GM-only section was touched by a player edit")
		assert.False(t, gmView.Document.Sections[1].GMOnly)
		assert.Equal(t, "The duke seems friendly.", gmView.Document.Sections[1].Content,
			"want visible appended section")
	}
}

func TestDocumentService_PlayerEdit_PreservesGMSectionsInPlace(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	_, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	doc := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path: "npcs/duke",
		Sections: []application.DocumentSectionInput{
			{Content: "GM intro.", GMOnly: true},
			{Content: "Public info."},
			{Content: "GM outro.", GMOnly: true},
		},
	})

	playerView, err := env.docs.Get(ctx, playerRequester, doc.Document.ID)
	require.NoError(t, err)

	_, err = env.docs.Update(ctx, playerRequester, doc.Document.ID, &application.UpdateDocumentInput{
		Path:  playerView.Document.Path,
		Title: playerView.Document.Title,
		Sections: []application.DocumentSectionInput{
			{ID: playerView.Document.Sections[0].ID, Content: "Public info, updated."},
		},
	})
	require.NoError(t, err)

	gmView, err := env.docs.Get(ctx, gmRequester, doc.Document.ID)
	require.NoError(t, err)

	want := []struct {
		content string
		gmOnly  bool
	}{
		{"GM intro.", true},
		{"Public info, updated.", false},
		{"GM outro.", true},
	}
	if assert.Len(t, gmView.Document.Sections, len(want)) {
		for i, w := range want {
			got := gmView.Document.Sections[i]
			assert.Equal(t, got.Content, w.content, "section %d content", i)
			assert.Equal(t, got.GMOnly, w.gmOnly, "section %d gmOnly", i)
		}
	}
}

func TestDocumentService_PlayerCannotChangeRevealDay(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	_, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	doc := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{Path: "lore/creation"})

	_, err = env.docs.Update(ctx, playerRequester, doc.Document.ID, &application.UpdateDocumentInput{
		Path:            doc.Document.Path,
		Title:           doc.Document.Title,
		SharedOnGameDay: 10,
	})
	require.ErrorIs(t, err, application.ErrForbidden)
}

func TestDocumentService_PlayerCannotReferenceGMOnlySection(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	_, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	doc := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path:     "npcs/duke",
		Sections: []application.DocumentSectionInput{{Content: "secret", GMOnly: true}},
	})

	gmView, err := env.docs.Get(ctx, gmRequester, doc.Document.ID)
	require.NoError(t, err)

	// A player addressing the GM-only row's ID gets the same error a garbage
	// ID would — its GM-ness never leaks.
	_, err = env.docs.Update(ctx, playerRequester, doc.Document.ID, &application.UpdateDocumentInput{
		Path:  doc.Document.Path,
		Title: doc.Document.Title,
		Sections: []application.DocumentSectionInput{
			{ID: gmView.Document.Sections[0].ID, Content: "overwritten"},
		},
	})
	require.ErrorIs(t, err, application.ErrInvalidDocument)
}

func TestDocumentService_CreateFromMarkdown_ParsesFrontmatter(t *testing.T) {
	env := newDocumentTestEnv(t)
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	markdown := "---\ntitle: The Thieves Guild\ntags: [faction, city]\ngame_day: 12\n---\n\nFull markdown content here..."
	view := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path:     "factions/thieves-guild",
		Markdown: markdown,
	})

	doc := view.Document
	assert.Equal(t, "The Thieves Guild", doc.Title)
	if assert.Len(t, doc.Tags, 2) {
		assert.Equal(t, "faction", doc.Tags[0])
		assert.Equal(t, "city", doc.Tags[1])
	}
	assert.Equal(t, 12, doc.SharedOnGameDay)
	if assert.Len(t, doc.Sections, 1, "want the body as one section") {
		assert.Equal(t, "Full markdown content here...", doc.Sections[0].Content)
	}
}

func TestDocumentService_RevealedFlag_TracksCharacterGameDays(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	doc := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path:            "reveals/the-betrayal",
		SharedOnGameDay: new(5),
	})

	view, err := env.docs.Get(ctx, gmRequester, doc.Document.ID)
	require.NoError(t, err)
	assert.False(t, view.Revealed, "before any character reached day 5")

	env.setGameDay(t, character.ID, 5)

	view, err = env.docs.Get(ctx, gmRequester, doc.Document.ID)
	require.NoError(t, err)
	assert.True(t, view.Revealed, "after a character reached day 5")
}

func TestDocumentService_FolderTree_NestsAndSortsAlphabetically(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{Path: "zebra"})
	mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{Path: "apple"})
	mustCreateDocument(t, env, gmRequester, general.ID,
		&application.CreateDocumentInput{Path: "factions/thieves-guild"})
	mustCreateDocument(t, env, gmRequester, general.ID,
		&application.CreateDocumentInput{Path: "factions/city-watch"})
	mustCreateDocument(t, env, gmRequester, general.ID,
		&application.CreateDocumentInput{Path: "factions/watch/roster"})

	tree, err := env.docs.FolderTree(ctx, gmRequester, general.ID)
	require.NoError(t, err)

	if assert.Len(t, tree.Documents, 2, "want [apple zebra]") {
		assert.Equal(t, "apple", tree.Documents[0].Title)
		assert.Equal(t, "zebra", tree.Documents[1].Title)
	}
	if assert.Len(t, tree.Folders, 1, "want [factions]") {
		assert.Equal(t, "factions", tree.Folders[0].Name)
	}

	factions := tree.Folders[0]
	assert.Equal(t, "factions", factions.Path)
	if assert.Len(t, factions.Documents, 2, "want [city-watch thieves-guild]") {
		assert.Equal(t, "city-watch", factions.Documents[0].Title)
		assert.Equal(t, "thieves-guild", factions.Documents[1].Title)
	}
	if assert.Len(t, factions.Folders, 1, "want [watch]") {
		assert.Equal(t, "watch", factions.Folders[0].Name)
		assert.Equal(t, "factions/watch", factions.Folders[0].Path)
		if assert.Len(t, factions.Folders[0].Documents, 1, "want [roster]") {
			assert.Equal(t, "roster", factions.Folders[0].Documents[0].Title)
		}
	}
}

func TestDocumentService_FolderTree_HidesFoldersWithNoAccessibleDocuments(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path:            "secrets/the-betrayal",
		SharedOnGameDay: new(5),
	})
	mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{Path: "lore/creation"})

	tree, err := env.docs.FolderTree(ctx, playerRequester, general.ID)
	require.NoError(t, err)

	if assert.Len(t, tree.Folders, 1, "the unrevealed secrets folder must not leak") {
		assert.Equal(t, "lore", tree.Folders[0].Name)
	}

	// Reaching the reveal day makes the previously-empty folder appear.
	env.setGameDay(t, character.ID, 5)

	tree, err = env.docs.FolderTree(ctx, playerRequester, general.ID)
	require.NoError(t, err)
	assert.Len(t, tree.Folders, 2, "want [lore secrets] after reveal")
}

func TestDocumentService_ShareToGroup_MovesDocumentAndAppliesGroupRules(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	group, err := env.groups.Create(ctx, gmRequester, "Thieves Guild", models.GroupTypeOrganization, "")
	require.NoError(t, err)
	err = env.groups.Join(ctx, gmRequester, group.ID, character.ID)
	require.NoError(t, err)

	charRepo := env.findRepository(t, models.RepositoryTypeCharacter, character.ID)
	groupRepo := env.findRepository(t, models.RepositoryTypeGroup, group.ID)

	doc := mustCreateDocument(t, env, playerRequester, charRepo.ID, &application.CreateDocumentInput{
		Path:     "notes/suspicions",
		Sections: []application.DocumentSectionInput{{Content: "I do not trust the duke."}},
	})

	shared, err := env.docs.ShareToGroup(ctx, playerRequester, doc.Document.ID, &application.ShareDocumentInput{
		TargetRepositoryID: groupRepo.ID,
		SharedOnGameDay:    3,
	})
	require.NoError(t, err)
	assert.Equal(t, shared.Document.RepositoryID, groupRepo.ID)
	assert.Equal(t, 3, shared.Document.SharedOnGameDay)

	// It's no longer in the character repository, and it now follows the
	// group's game-day gate: the owner's own character hasn't reached day 3
	// yet, so it isn't visible until it does.
	listed, err := env.docs.ListByRepository(ctx, playerRequester, charRepo.ID)
	require.NoError(t, err)
	assert.Empty(t, listed)
	_, err = env.docs.Get(ctx, playerRequester, doc.Document.ID)
	require.ErrorIs(t, err, application.ErrNotFound)
	env.setGameDay(t, character.ID, 3)
	_, err = env.docs.Get(ctx, playerRequester, doc.Document.ID)
	require.NoError(t, err)

	// Game-day gating now applies via the group: a fellow member below day 3
	// sees nothing yet.
	otherRequester := fakeRequester{id: "other-1", gm: false}
	other, err := env.characters.Create(ctx, otherRequester, "", "Beren")
	require.NoError(t, err)
	err = env.groups.Join(ctx, gmRequester, group.ID, other.ID)
	require.NoError(t, err)

	_, err = env.docs.Get(ctx, otherRequester, doc.Document.ID)
	require.ErrorIs(t, err, application.ErrNotFound)

	env.setGameDay(t, other.ID, 3)

	_, err = env.docs.Get(ctx, otherRequester, doc.Document.ID)
	require.NoError(t, err)
}

func TestDocumentService_ShareToGroup_NonMemberCannotShareIntoGroup(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	group, err := env.groups.Create(ctx, gmRequester, "Thieves Guild", models.GroupTypeOrganization, "")
	require.NoError(t, err)
	// Note: character never joins the group.

	charRepo := env.findRepository(t, models.RepositoryTypeCharacter, character.ID)
	groupRepo := env.findRepository(t, models.RepositoryTypeGroup, group.ID)

	doc := mustCreateDocument(t, env, playerRequester, charRepo.ID, &application.CreateDocumentInput{
		Path: "notes/suspicions",
	})

	_, err = env.docs.ShareToGroup(ctx, playerRequester, doc.Document.ID, &application.ShareDocumentInput{
		TargetRepositoryID: groupRepo.ID,
		SharedOnGameDay:    1,
	})
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestDocumentService_ShareToGroup_OtherPlayerCannotShareSomeoneElsesDocument(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	other := fakeRequester{id: "other-1", gm: false}
	_, err = env.characters.Create(ctx, other, "", "Beren")
	require.NoError(t, err)

	group, err := env.groups.Create(ctx, gmRequester, "Thieves Guild", models.GroupTypeOrganization, "")
	require.NoError(t, err)

	charRepo := env.findRepository(t, models.RepositoryTypeCharacter, character.ID)
	groupRepo := env.findRepository(t, models.RepositoryTypeGroup, group.ID)

	doc := mustCreateDocument(t, env, playerRequester, charRepo.ID, &application.CreateDocumentInput{
		Path: "notes/suspicions",
	})

	_, err = env.docs.ShareToGroup(ctx, other, doc.Document.ID, &application.ShareDocumentInput{
		TargetRepositoryID: groupRepo.ID,
		SharedOnGameDay:    1,
	})
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestDocumentService_ShareToGroup_OnlyFromCharacterRepository(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	group, err := env.groups.Create(ctx, gmRequester, "Thieves Guild", models.GroupTypeOrganization, "")
	require.NoError(t, err)
	groupRepo := env.findRepository(t, models.RepositoryTypeGroup, group.ID)

	doc := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{Path: "lore/creation"})

	_, err = env.docs.ShareToGroup(ctx, gmRequester, doc.Document.ID, &application.ShareDocumentInput{
		TargetRepositoryID: groupRepo.ID,
		SharedOnGameDay:    1,
	})
	require.ErrorIs(t, err, application.ErrInvalidDocument)
}

func TestDocumentService_ShareToGroup_TargetMustBeGroupRepository(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)
	charRepo := env.findRepository(t, models.RepositoryTypeCharacter, character.ID)

	doc := mustCreateDocument(t, env, playerRequester, charRepo.ID, &application.CreateDocumentInput{
		Path: "notes/suspicions",
	})

	_, err = env.docs.ShareToGroup(ctx, playerRequester, doc.Document.ID, &application.ShareDocumentInput{
		TargetRepositoryID: general.ID,
		SharedOnGameDay:    1,
	})
	require.ErrorIs(t, err, application.ErrInvalidDocument)
}

func TestDocumentService_ShareToGroup_PathCollision_WarnsThenAllows(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	group, err := env.groups.Create(ctx, gmRequester, "Thieves Guild", models.GroupTypeOrganization, "")
	require.NoError(t, err)
	err = env.groups.Join(ctx, gmRequester, group.ID, character.ID)
	require.NoError(t, err)

	charRepo := env.findRepository(t, models.RepositoryTypeCharacter, character.ID)
	groupRepo := env.findRepository(t, models.RepositoryTypeGroup, group.ID)

	mustCreateDocument(t, env, gmRequester, groupRepo.ID, &application.CreateDocumentInput{Path: "notes/suspicions"})
	doc := mustCreateDocument(t, env, playerRequester, charRepo.ID, &application.CreateDocumentInput{
		Path: "notes/suspicions",
	})

	_, err = env.docs.ShareToGroup(ctx, playerRequester, doc.Document.ID, &application.ShareDocumentInput{
		TargetRepositoryID: groupRepo.ID,
		SharedOnGameDay:    1,
	})
	require.ErrorIs(t, err, application.ErrPathCollision)

	shared, err := env.docs.ShareToGroup(ctx, playerRequester, doc.Document.ID, &application.ShareDocumentInput{
		TargetRepositoryID: groupRepo.ID,
		SharedOnGameDay:    1,
		AllowCollision:     true,
	})
	require.NoError(t, err)
	assert.Equal(t, shared.Document.RepositoryID, groupRepo.ID)
}

func TestDocumentService_DirectShare_GatesByCharacterGameDay(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()

	owner, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	other := fakeRequester{id: "other-1", gm: false}
	recipient, err := env.characters.Create(ctx, other, "", "Beren")
	require.NoError(t, err)

	charRepo := env.findRepository(t, models.RepositoryTypeCharacter, owner.ID)
	doc := mustCreateDocument(t, env, playerRequester, charRepo.ID, &application.CreateDocumentInput{
		Path:     "notes/heirloom",
		Sections: []application.DocumentSectionInput{{Content: "The ring is cursed."}},
	})

	// The recipient's own character repository grants nothing here.
	_, err = env.docs.Get(ctx, other, doc.Document.ID)
	require.ErrorIs(t, err, application.ErrNotFound)

	share, err := env.docs.ShareWithCharacter(ctx, gmRequester, doc.Document.ID, recipient.ID, 3)
	require.NoError(t, err)

	// Shared, but the character's game day hasn't reached the share's yet.
	_, err = env.docs.Get(ctx, other, doc.Document.ID)
	require.ErrorIs(t, err, application.ErrNotFound)

	env.setGameDay(t, recipient.ID, 3)

	view, err := env.docs.Get(ctx, other, doc.Document.ID)
	require.NoError(t, err)
	if assert.Len(t, view.Document.Sections, 1, "want the shared content") {
		assert.Equal(t, "The ring is cursed.", view.Document.Sections[0].Content)
	}

	// A second share attempt for the same pair is rejected.
	_, err = env.docs.ShareWithCharacter(ctx, gmRequester, doc.Document.ID, recipient.ID, 3)
	require.ErrorIs(t, err, application.ErrAlreadyShared)

	// Revoking removes access again.
	err = env.docs.RevokeShare(ctx, gmRequester, doc.Document.ID, share.ID)
	require.NoError(t, err)
	_, err = env.docs.Get(ctx, other, doc.Document.ID)
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestDocumentService_DirectShare_GMOnlySectionsStrippedForRecipient(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	recipient := fakeRequester{id: "recipient-1", gm: false}
	character, err := env.characters.Create(ctx, recipient, "", "Beren")
	require.NoError(t, err)

	doc := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path:            "npcs/duke",
		SharedOnGameDay: new(100), // Far past the recipient's game day: repo path stays gated.
		Sections: []application.DocumentSectionInput{
			{Content: "The duke rules the city."},
			{Content: "He is secretly a vampire.", GMOnly: true},
		},
	})

	_, err = env.docs.ShareWithCharacter(ctx, gmRequester, doc.Document.ID, character.ID, 0)
	require.NoError(t, err)

	view, err := env.docs.Get(ctx, recipient, doc.Document.ID)
	require.NoError(t, err)
	if assert.Len(t, view.Document.Sections, 1, "want only the non-GM section") {
		assert.False(t, view.Document.Sections[0].GMOnly)
	}
}

func TestDocumentService_ShareWithCharacter_ForbiddenForPlayers(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	doc := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{Path: "lore/creation"})

	_, err = env.docs.ShareWithCharacter(ctx, playerRequester, doc.Document.ID, character.ID, 0)
	require.ErrorIs(t, err, application.ErrForbidden)
}

func TestDocumentService_ListSharedWithMe(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()

	owner, err := env.characters.Create(ctx, gmRequester, "", "Aria")
	require.NoError(t, err)

	recipient := fakeRequester{id: "recipient-1", gm: false}
	character, err := env.characters.Create(ctx, recipient, "", "Beren")
	require.NoError(t, err)

	charRepo := env.findRepository(t, models.RepositoryTypeCharacter, owner.ID)
	doc := mustCreateDocument(t, env, gmRequester, charRepo.ID, &application.CreateDocumentInput{
		Path: "notes/heirloom",
	})

	views, err := env.docs.ListSharedWithMe(ctx, recipient)
	require.NoError(t, err)
	assert.Empty(t, views, "want none before sharing")

	_, err = env.docs.ShareWithCharacter(ctx, gmRequester, doc.Document.ID, character.ID, 0)
	require.NoError(t, err)

	views, err = env.docs.ListSharedWithMe(ctx, recipient)
	require.NoError(t, err)
	if assert.Len(t, views, 1) {
		assert.Equal(t, views[0].Document.ID, doc.Document.ID)
	}
}
