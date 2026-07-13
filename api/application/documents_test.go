package application_test

import (
	"errors"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
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
	docSvc := application.NewDocumentService(repositories.NewDocuments(db), repoSvc, characterRepo, groupRepo)

	if err := repoSvc.EnsureSystemRepositories(t.Context()); err != nil {
		t.Fatalf("EnsureSystemRepositories: %v", err)
	}

	return documentTestEnv{docs: docSvc, repos: repoSvc, characters: charSvc, groups: groupSvc}
}

// findRepository returns the one repository matching the type and (for group/
// character repositories) the owner ID.
func (e documentTestEnv) findRepository(
	t *testing.T, repoType models.RepositoryType, ownerID string,
) *models.Repository {
	t.Helper()

	repos, err := e.repos.List(t.Context(), gmRequester)
	if err != nil {
		t.Fatalf("List repositories: %v", err)
	}

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

	t.Fatalf("no %s repository found for owner %q", repoType, ownerID)

	return nil
}

func (e documentTestEnv) setGameDay(t *testing.T, characterID string, day int) {
	t.Helper()

	if _, err := e.characters.Update(t.Context(), gmRequester, characterID, nil, &day); err != nil {
		t.Fatalf("set game day: %v", err)
	}
}

func mustCreateDocument(
	t *testing.T, env documentTestEnv,
	requester application.Requester, repoID string, input *application.CreateDocumentInput,
) *application.DocumentView {
	t.Helper()

	view, err := env.docs.Create(t.Context(), requester, repoID, input)
	if err != nil {
		t.Fatalf("Create document: %v", err)
	}

	return view
}

func TestDocumentService_CreateAndGet_TitleFallsBackToFileName(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	view := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path:     "factions//thieves-guild",
		Sections: []application.DocumentSectionInput{{Content: "The guild runs the docks."}},
	})

	if view.Document.Path != "factions/thieves-guild" {
		t.Fatalf("Path = %q, want normalized %q", view.Document.Path, "factions/thieves-guild")
	}
	if view.Document.Title != "thieves-guild" {
		t.Fatalf("Title = %q, want file-name fallback %q", view.Document.Title, "thieves-guild")
	}

	got, err := env.docs.Get(ctx, gmRequester, view.Document.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Document.Sections) != 1 || got.Document.Sections[0].Content != "The guild runs the docks." {
		t.Fatalf("Sections = %+v, want the created section back", got.Document.Sections)
	}
}

func TestDocumentService_GameDayGatesVisibility_IncludingRewind(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	if err != nil {
		t.Fatalf("Create character: %v", err)
	}

	doc := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path:            "reveals/the-betrayal",
		SharedOnGameDay: intPtr(5),
		Sections:        []application.DocumentSectionInput{{Content: "He was the traitor all along."}},
	})

	// Day 0 < 5: absent from lists AND a direct Get is a 404, never a 403.
	if _, err := env.docs.Get(ctx, playerRequester, doc.Document.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Get before game day = %v, want ErrNotFound", err)
	}

	listed, err := env.docs.ListByRepository(ctx, playerRequester, general.ID)
	if err != nil {
		t.Fatalf("ListByRepository: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("List before game day returned %d documents, want 0", len(listed))
	}

	// Reaching day 5 reveals it.
	env.setGameDay(t, character.ID, 5)

	if _, err := env.docs.Get(ctx, playerRequester, doc.Document.ID); err != nil {
		t.Fatalf("Get at game day = %v, want success", err)
	}

	// Rewinding hides it again.
	env.setGameDay(t, character.ID, 4)

	if _, err := env.docs.Get(ctx, playerRequester, doc.Document.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Get after rewind = %v, want ErrNotFound", err)
	}
}

func TestDocumentService_GroupRepository_HiddenFromNonMembers(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()

	member, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	if err != nil {
		t.Fatalf("Create member character: %v", err)
	}

	outsider := fakeRequester{id: "outsider-1", gm: false}
	if _, err := env.characters.Create(ctx, outsider, "", "Beren"); err != nil {
		t.Fatalf("Create outsider character: %v", err)
	}

	group, err := env.groups.Create(ctx, gmRequester, "Thieves Guild", models.GroupTypeOrganization, "")
	if err != nil {
		t.Fatalf("Create group: %v", err)
	}
	if err := env.groups.Join(ctx, gmRequester, group.ID, member.ID); err != nil {
		t.Fatalf("Join: %v", err)
	}

	groupRepo := env.findRepository(t, models.RepositoryTypeGroup, group.ID)
	doc := mustCreateDocument(t, env, gmRequester, groupRepo.ID, &application.CreateDocumentInput{
		Path:     "plans/heist",
		Sections: []application.DocumentSectionInput{{Content: "We strike at midnight."}},
	})

	if _, err := env.docs.Get(ctx, playerRequester, doc.Document.ID); err != nil {
		t.Fatalf("member Get = %v, want success", err)
	}

	if _, err := env.docs.Get(ctx, outsider, doc.Document.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("outsider Get = %v, want ErrNotFound", err)
	}
	if _, err := env.docs.ListByRepository(ctx, outsider, groupRepo.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("outsider ListByRepository = %v, want ErrNotFound", err)
	}
}

func TestDocumentService_CharacterRepository_HiddenFromOtherPlayers(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	if err != nil {
		t.Fatalf("Create character: %v", err)
	}

	other := fakeRequester{id: "other-1", gm: false}
	if _, err := env.characters.Create(ctx, other, "", "Beren"); err != nil {
		t.Fatalf("Create other character: %v", err)
	}

	charRepo := env.findRepository(t, models.RepositoryTypeCharacter, character.ID)
	doc := mustCreateDocument(t, env, playerRequester, charRepo.ID, &application.CreateDocumentInput{
		Path:     "notes/suspicions",
		Sections: []application.DocumentSectionInput{{Content: "I do not trust the duke."}},
	})

	if _, err := env.docs.Get(ctx, playerRequester, doc.Document.ID); err != nil {
		t.Fatalf("owner Get = %v, want success", err)
	}
	if _, err := env.docs.Get(ctx, gmRequester, doc.Document.ID); err != nil {
		t.Fatalf("GM Get = %v, want success", err)
	}

	if _, err := env.docs.Get(ctx, other, doc.Document.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("other player Get = %v, want ErrNotFound", err)
	}
}

func TestDocumentService_GMOnlySections_StrippedForPlayers(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	if _, err := env.characters.Create(ctx, playerRequester, "", "Aria"); err != nil {
		t.Fatalf("Create character: %v", err)
	}

	doc := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path: "npcs/duke",
		Sections: []application.DocumentSectionInput{
			{Content: "The duke rules the city."},
			{Content: "He is secretly a vampire.", GMOnly: true},
		},
	})

	playerView, err := env.docs.Get(ctx, playerRequester, doc.Document.ID)
	if err != nil {
		t.Fatalf("player Get: %v", err)
	}
	if len(playerView.Document.Sections) != 1 {
		t.Fatalf("player sees %d sections, want 1", len(playerView.Document.Sections))
	}
	for _, sec := range playerView.Document.Sections {
		if sec.GMOnly || sec.Content == "He is secretly a vampire." {
			t.Fatalf("GM-only content leaked to player: %+v", sec)
		}
	}

	gmView, err := env.docs.Get(ctx, gmRequester, doc.Document.ID)
	if err != nil {
		t.Fatalf("GM Get: %v", err)
	}
	if len(gmView.Document.Sections) != 2 {
		t.Fatalf("GM sees %d sections, want 2", len(gmView.Document.Sections))
	}
}

func TestDocumentService_PlayerCannotCreateGMOnlySection(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	if _, err := env.characters.Create(ctx, playerRequester, "", "Aria"); err != nil {
		t.Fatalf("Create character: %v", err)
	}

	_, err := env.docs.Create(ctx, playerRequester, general.ID, &application.CreateDocumentInput{
		Path:     "npcs/duke",
		Sections: []application.DocumentSectionInput{{Content: "secret", GMOnly: true}},
	})
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("Create = %v, want ErrForbidden", err)
	}
}

func TestDocumentService_PathCollision_WarnsThenAllows(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path: "lore/creation",
	})

	_, err := env.docs.Create(ctx, gmRequester, general.ID, &application.CreateDocumentInput{Path: "lore/creation"})
	if !errors.Is(err, application.ErrPathCollision) {
		t.Fatalf("colliding Create = %v, want ErrPathCollision", err)
	}

	if _, err := env.docs.Create(ctx, gmRequester, general.ID, &application.CreateDocumentInput{
		Path: "lore/creation", AllowCollision: true,
	}); err != nil {
		t.Fatalf("Create with AllowCollision = %v, want success", err)
	}
}

func TestDocumentService_PathCollision_OnMove(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{Path: "lore/creation"})
	doc := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{Path: "lore/draft"})

	update := application.UpdateDocumentInput{Path: "lore/creation", Title: doc.Document.Title}
	if _, err := env.docs.Update(ctx, gmRequester, doc.Document.ID, &update); !errors.Is(err, application.ErrPathCollision) {
		t.Fatalf("moving onto occupied path = %v, want ErrPathCollision", err)
	}

	update.AllowCollision = true
	if _, err := env.docs.Update(ctx, gmRequester, doc.Document.ID, &update); err != nil {
		t.Fatalf("move with AllowCollision = %v, want success", err)
	}
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
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	staleVersion := loaded.Document.Version

	// First save with the loaded version succeeds.
	first := application.UpdateDocumentInput{
		Path:            loaded.Document.Path,
		Title:           loaded.Document.Title,
		Sections:        []application.DocumentSectionInput{{ID: loaded.Document.Sections[0].ID, Content: "v2"}},
		ExpectedVersion: &staleVersion,
	}
	if _, err := env.docs.Update(ctx, gmRequester, created.Document.ID, &first); err != nil {
		t.Fatalf("first Update = %v, want success", err)
	}

	// A second save still holding the old version warns.
	second := first
	second.Sections = []application.DocumentSectionInput{{ID: loaded.Document.Sections[0].ID, Content: "v3"}}
	if _, err := env.docs.Update(ctx, gmRequester, created.Document.ID, &second); !errors.Is(err, application.ErrConcurrentEdit) {
		t.Fatalf("stale Update = %v, want ErrConcurrentEdit", err)
	}

	// Forcing overwrites anyway.
	second.Force = true
	updated, err := env.docs.Update(ctx, gmRequester, created.Document.ID, &second)
	if err != nil {
		t.Fatalf("forced Update = %v, want success", err)
	}
	if updated.Document.Sections[0].Content != "v3" {
		t.Fatalf("Content = %q, want %q", updated.Document.Sections[0].Content, "v3")
	}
}

func TestDocumentService_PlayerEditOnAllGMOnlyDocument_AppendsVisibleSection(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	if _, err := env.characters.Create(ctx, playerRequester, "", "Aria"); err != nil {
		t.Fatalf("Create character: %v", err)
	}

	doc := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path:     "npcs/duke",
		Sections: []application.DocumentSectionInput{{Content: "He is secretly a vampire.", GMOnly: true}},
	})

	// The player opens an apparently empty document and writes into it.
	playerView, err := env.docs.Get(ctx, playerRequester, doc.Document.ID)
	if err != nil {
		t.Fatalf("player Get: %v", err)
	}
	if len(playerView.Document.Sections) != 0 {
		t.Fatalf("player sees %d sections, want 0", len(playerView.Document.Sections))
	}

	_, err = env.docs.Update(ctx, playerRequester, doc.Document.ID, &application.UpdateDocumentInput{
		Path:     playerView.Document.Path,
		Title:    playerView.Document.Title,
		Sections: []application.DocumentSectionInput{{Content: "The duke seems friendly."}},
	})
	if err != nil {
		t.Fatalf("player Update = %v, want success", err)
	}

	gmView, err := env.docs.Get(ctx, gmRequester, doc.Document.ID)
	if err != nil {
		t.Fatalf("GM Get: %v", err)
	}
	if len(gmView.Document.Sections) != 2 {
		t.Fatalf("GM sees %d sections, want GM section + new player section", len(gmView.Document.Sections))
	}
	if !gmView.Document.Sections[0].GMOnly || gmView.Document.Sections[0].Content != "He is secretly a vampire." {
		t.Fatalf("GM-only section was touched by a player edit: %+v", gmView.Document.Sections[0])
	}
	if gmView.Document.Sections[1].GMOnly || gmView.Document.Sections[1].Content != "The duke seems friendly." {
		t.Fatalf("player section = %+v, want visible appended section", gmView.Document.Sections[1])
	}
}

func TestDocumentService_PlayerEdit_PreservesGMSectionsInPlace(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	if _, err := env.characters.Create(ctx, playerRequester, "", "Aria"); err != nil {
		t.Fatalf("Create character: %v", err)
	}

	doc := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path: "npcs/duke",
		Sections: []application.DocumentSectionInput{
			{Content: "GM intro.", GMOnly: true},
			{Content: "Public info."},
			{Content: "GM outro.", GMOnly: true},
		},
	})

	playerView, err := env.docs.Get(ctx, playerRequester, doc.Document.ID)
	if err != nil {
		t.Fatalf("player Get: %v", err)
	}

	_, err = env.docs.Update(ctx, playerRequester, doc.Document.ID, &application.UpdateDocumentInput{
		Path:  playerView.Document.Path,
		Title: playerView.Document.Title,
		Sections: []application.DocumentSectionInput{
			{ID: playerView.Document.Sections[0].ID, Content: "Public info, updated."},
		},
	})
	if err != nil {
		t.Fatalf("player Update = %v, want success", err)
	}

	gmView, err := env.docs.Get(ctx, gmRequester, doc.Document.ID)
	if err != nil {
		t.Fatalf("GM Get: %v", err)
	}

	want := []struct {
		content string
		gmOnly  bool
	}{
		{"GM intro.", true},
		{"Public info, updated.", false},
		{"GM outro.", true},
	}
	if len(gmView.Document.Sections) != len(want) {
		t.Fatalf("GM sees %d sections, want %d", len(gmView.Document.Sections), len(want))
	}
	for i, w := range want {
		got := gmView.Document.Sections[i]
		if got.Content != w.content || got.GMOnly != w.gmOnly {
			t.Fatalf("section %d = %+v, want content %q gmOnly %t", i, got, w.content, w.gmOnly)
		}
	}
}

func TestDocumentService_PlayerCannotChangeRevealDay(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	if _, err := env.characters.Create(ctx, playerRequester, "", "Aria"); err != nil {
		t.Fatalf("Create character: %v", err)
	}

	doc := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{Path: "lore/creation"})

	_, err := env.docs.Update(ctx, playerRequester, doc.Document.ID, &application.UpdateDocumentInput{
		Path:            doc.Document.Path,
		Title:           doc.Document.Title,
		SharedOnGameDay: 10,
	})
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("player reveal-day change = %v, want ErrForbidden", err)
	}
}

func TestDocumentService_PlayerCannotReferenceGMOnlySection(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	if _, err := env.characters.Create(ctx, playerRequester, "", "Aria"); err != nil {
		t.Fatalf("Create character: %v", err)
	}

	doc := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path:     "npcs/duke",
		Sections: []application.DocumentSectionInput{{Content: "secret", GMOnly: true}},
	})

	gmView, err := env.docs.Get(ctx, gmRequester, doc.Document.ID)
	if err != nil {
		t.Fatalf("GM Get: %v", err)
	}

	// A player addressing the GM-only row's ID gets the same error a garbage
	// ID would — its GM-ness never leaks.
	_, err = env.docs.Update(ctx, playerRequester, doc.Document.ID, &application.UpdateDocumentInput{
		Path:  doc.Document.Path,
		Title: doc.Document.Title,
		Sections: []application.DocumentSectionInput{
			{ID: gmView.Document.Sections[0].ID, Content: "overwritten"},
		},
	})
	if !errors.Is(err, application.ErrInvalidDocument) {
		t.Fatalf("player Update referencing GM section = %v, want ErrInvalidDocument", err)
	}
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
	if doc.Title != "The Thieves Guild" {
		t.Fatalf("Title = %q, want frontmatter title", doc.Title)
	}
	if len(doc.Tags) != 2 || doc.Tags[0] != "faction" || doc.Tags[1] != "city" {
		t.Fatalf("Tags = %v, want [faction city]", doc.Tags)
	}
	if doc.SharedOnGameDay != 12 {
		t.Fatalf("SharedOnGameDay = %d, want 12", doc.SharedOnGameDay)
	}
	if len(doc.Sections) != 1 || doc.Sections[0].Content != "Full markdown content here..." {
		t.Fatalf("Sections = %+v, want the body as one section", doc.Sections)
	}
}

func TestDocumentService_RevealedFlag_TracksCharacterGameDays(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	if err != nil {
		t.Fatalf("Create character: %v", err)
	}

	doc := mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path:            "reveals/the-betrayal",
		SharedOnGameDay: intPtr(5),
	})

	view, err := env.docs.Get(ctx, gmRequester, doc.Document.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if view.Revealed {
		t.Fatalf("Revealed = true before any character reached day 5")
	}

	env.setGameDay(t, character.ID, 5)

	view, err = env.docs.Get(ctx, gmRequester, doc.Document.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !view.Revealed {
		t.Fatalf("Revealed = false after a character reached day 5")
	}
}

func intPtr(v int) *int { return &v }
