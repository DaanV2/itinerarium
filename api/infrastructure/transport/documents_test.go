package transport_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
	"github.com/stretchr/testify/require"
)

type documentsTransportEnv struct {
	router         *transport.Router
	gmToken        string
	playerToken    string
	generalID      string
	characterID    string
	groups         *application.GroupService
	repositories   *application.RepositoryService
	gmRequester    application.Requester
	characters     *repositories.Characters
	knowledgeRepos *repositories.KnowledgeRepositories
}

func newDocumentsTransportEnv(t *testing.T) documentsTransportEnv {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err)
	err = db.Migrate()
	require.NoError(t, err)

	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(t.TempDir()))
	require.NoError(t, err)

	tokens := authentication.NewTokenService(keys, repositories.NewRevokedTokens(db))
	users := repositories.NewUsers(db)
	characters := repositories.NewCharacters(db)
	groups := repositories.NewGroups(db)
	knowledgeRepos := repositories.NewKnowledgeRepositories(db)

	authSvc := application.NewAuthService(tokens, users)
	repoSvc := application.NewRepositoryService(knowledgeRepos, groups, characters)
	charSvc := application.NewCharacterService(characters, users, knowledgeRepos)
	groupSvc := application.NewGroupService(groups, charSvc, knowledgeRepos)
	docSvc := application.NewDocumentService(
		repositories.NewDocuments(db), repoSvc, characters, groups, repositories.NewDocumentShares(db),
	)
	requireAuth := transport.RequireAuth(authSvc)

	ctx := t.Context()

	err = repoSvc.EnsureSystemRepositories(ctx)
	require.NoError(t, err)

	general, err := knowledgeRepos.EnsureGeneral(ctx)
	require.NoError(t, err)

	gm := &models.User{Email: "gm@example.com", PasswordHash: "hash", Role: models.RoleGM}
	err = users.Create(ctx, gm)
	require.NoError(t, err)

	player := &models.User{Email: "player@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	err = users.Create(ctx, player)
	require.NoError(t, err)

	gmToken, err := tokens.Issue(gm.ID)
	require.NoError(t, err)

	playerToken, err := tokens.Issue(player.ID)
	require.NoError(t, err)

	// The player needs a character: game-day gating resolves through it, and
	// CharacterService.Create also provisions its private repository.
	character, err := charSvc.Create(ctx, application.UserRequester{User: player}, "", "Aria")
	require.NoError(t, err)

	router := transport.NewRouter(
		transport.WithHandle(
			"GET /api/repositories/{id}/documents", requireAuth(transport.ListDocumentsHandler(docSvc)),
		),
		transport.WithHandle(
			"POST /api/repositories/{id}/documents", requireAuth(transport.CreateDocumentHandler(docSvc)),
		),
		transport.WithHandle(
			"GET /api/repositories/{id}/documents/tree", requireAuth(transport.GetDocumentFolderTreeHandler(docSvc)),
		),
		transport.WithHandle("GET /api/documents/shared", requireAuth(transport.ListSharedDocumentsHandler(docSvc))),
		transport.WithHandle("GET /api/documents/{id}", requireAuth(transport.GetDocumentHandler(docSvc))),
		transport.WithHandle("PATCH /api/documents/{id}", requireAuth(transport.UpdateDocumentHandler(docSvc))),
		transport.WithHandle("POST /api/documents/{id}/share", requireAuth(transport.ShareDocumentHandler(docSvc))),
		transport.WithHandle("GET /api/documents/{id}/shares", requireAuth(transport.ListDocumentSharesHandler(docSvc))),
		transport.WithHandle(
			"POST /api/documents/{id}/shares", requireAuth(transport.ShareDocumentWithCharacterHandler(docSvc)),
		),
		transport.WithHandle(
			"DELETE /api/documents/{id}/shares/{shareId}", requireAuth(transport.RevokeDocumentShareHandler(docSvc)),
		),
	)

	return documentsTransportEnv{
		router:         router,
		gmToken:        gmToken,
		playerToken:    playerToken,
		generalID:      general.ID,
		characterID:    character.ID,
		groups:         groupSvc,
		repositories:   repoSvc,
		gmRequester:    application.UserRequester{User: gm},
		characters:     characters,
		knowledgeRepos: knowledgeRepos,
	}
}

// findRepositoryID returns the ID of the one repository matching repoType
// and (for group/character repositories) the given owner ID.
func (e *documentsTransportEnv) findRepositoryID(t *testing.T, repoType models.RepositoryType, ownerID string) string {
	t.Helper()

	repos, err := e.repositories.List(t.Context(), e.gmRequester)
	require.NoError(t, err)

	for i := range repos {
		r := &repos[i]
		if r.Type != repoType {
			continue
		}

		switch repoType {
		case models.RepositoryTypeGroup:
			if r.GroupID != nil && *r.GroupID == ownerID {
				return r.ID
			}
		case models.RepositoryTypeCharacter:
			if r.CharacterID != nil && *r.CharacterID == ownerID {
				return r.ID
			}
		case models.RepositoryTypeGeneral, models.RepositoryTypeTemplate:
			return r.ID
		}
	}

	t.Fatalf("no %s repository found for owner %q", repoType, ownerID)

	return ""
}

// createOtherCharacterRepository creates a second player's character (owned
// by neither the GM nor env's player) and returns its private character
// repository ID — a repository the test player has no access to.
func (e *documentsTransportEnv) createOtherCharacterRepository(t *testing.T) string {
	t.Helper()

	ctx := t.Context()
	other := &models.Character{Name: "Beren", UserID: "other-user"}
	err := e.characters.Create(ctx, other)
	require.NoError(t, err)

	repo, err := e.knowledgeRepos.EnsureForCharacter(ctx, other.ID)
	require.NoError(t, err)

	return repo.ID
}

// createDocumentIn creates a document in the given repository as the GM.
func (e *documentsTransportEnv) createDocumentIn(t *testing.T, repositoryID string, body map[string]any) map[string]any {
	t.Helper()

	rec := e.do(t, http.MethodPost, "/api/repositories/"+repositoryID+"/documents", e.gmToken, body)
	require.Equal(t, rec.Code, http.StatusCreated)

	var doc map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &doc)
	require.NoError(t, err)

	return doc
}

// setCharacterGameDay sets env's player character's current_game_day.
func (e *documentsTransportEnv) setCharacterGameDay(t *testing.T, day int) {
	t.Helper()

	ctx := t.Context()
	character, err := e.characters.GetByID(ctx, e.characterID)
	require.NoError(t, err)

	character.CurrentGameDay = day
	err = e.characters.Update(ctx, character)
	require.NoError(t, err)
}

func (e *documentsTransportEnv) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader = http.NoBody
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}

		reader = bytes.NewReader(encoded)
	}

	req := httptest.NewRequestWithContext(t.Context(), method, path, reader)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)

	return rec
}

func (e *documentsTransportEnv) createDocument(t *testing.T, body map[string]any) map[string]any {
	t.Helper()

	rec := e.do(t, http.MethodPost, "/api/repositories/"+e.generalID+"/documents", e.gmToken, body)
	require.Equal(t, rec.Code, http.StatusCreated)

	var doc map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &doc)
	require.NoError(t, err)

	return doc
}

func TestDocumentRoutes_GMOnlyContentNeverReachesPlayers(t *testing.T) {
	env := newDocumentsTransportEnv(t)

	doc := env.createDocument(t, map[string]any{
		"path": "npcs/duke",
		"sections": []map[string]any{
			{"content": "The duke rules the city."},
			{"content": "He is secretly a vampire.", "gm_only": true},
		},
	})

	docID, _ := doc["id"].(string)
	rec := env.do(t, http.MethodGet, "/api/documents/"+docID, env.playerToken, nil)
	require.Equal(t, rec.Code, http.StatusOK)

	// Grep the raw payload, not the parsed struct: nothing GM-only may be in
	// the bytes a player receives.
	payload := rec.Body.String()
	if strings.Contains(payload, "vampire") || strings.Contains(payload, `"gm_only":true`) {
		t.Fatalf("GM-only content leaked into player payload: %s", payload)
	}
}

func TestDocumentRoutes_GameDayGate_Returns404NotForbidden(t *testing.T) {
	env := newDocumentsTransportEnv(t)

	doc := env.createDocument(t, map[string]any{
		"path":               "reveals/the-betrayal",
		"shared_on_game_day": 5,
	})

	docID, _ := doc["id"].(string)
	rec := env.do(t, http.MethodGet, "/api/documents/"+docID, env.playerToken, nil)
	require.Equal(t, rec.Code, http.StatusNotFound)

	list := env.do(t, http.MethodGet, "/api/repositories/"+env.generalID+"/documents", env.playerToken, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", list.Code)
	}
	if body := list.Body.String(); strings.Contains(body, "the-betrayal") {
		t.Fatalf("gated document leaked into list: %s", body)
	}
}

func TestDocumentRoutes_FolderTree_HidesGatedFolder(t *testing.T) {
	env := newDocumentsTransportEnv(t)

	env.createDocument(t, map[string]any{"path": "lore/creation"})
	env.createDocument(t, map[string]any{
		"path":               "secrets/the-betrayal",
		"shared_on_game_day": 5,
	})

	rec := env.do(t, http.MethodGet, "/api/repositories/"+env.generalID+"/documents/tree", env.playerToken, nil)
	require.Equal(t, rec.Code, http.StatusOK)
	var tree struct {
		Folders []struct {
			Name string `json:"name"`
		} `json:"folders"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &tree)
	require.NoError(t, err)

	if len(tree.Folders) != 1 || tree.Folders[0].Name != "lore" {
		t.Fatalf("folders = %+v, want [lore] — the unrevealed secrets folder must not leak", tree.Folders)
	}
}

func TestDocumentRoutes_PathCollision_409WithCode(t *testing.T) {
	env := newDocumentsTransportEnv(t)

	env.createDocument(t, map[string]any{"path": "lore/creation"})

	rec := env.do(t, http.MethodPost, "/api/repositories/"+env.generalID+"/documents", env.gmToken,
		map[string]any{"path": "lore/creation"})
	require.Equal(t, rec.Code, http.StatusConflict)
	if !strings.Contains(rec.Body.String(), `"code":"path_collision"`) {
		t.Fatalf("collision body missing code: %s", rec.Body.String())
	}

	allowed := env.do(t, http.MethodPost, "/api/repositories/"+env.generalID+"/documents", env.gmToken,
		map[string]any{"path": "lore/creation", "allow_collision": true})
	if allowed.Code != http.StatusCreated {
		t.Fatalf("allow_collision create status = %d, want 201", allowed.Code)
	}
}

func TestDocumentRoutes_ConcurrentEdit_409WithCode(t *testing.T) {
	env := newDocumentsTransportEnv(t)

	doc := env.createDocument(t, map[string]any{"path": "lore/creation"})
	docID, _ := doc["id"].(string)
	version, _ := doc["version"].(float64)

	update := map[string]any{
		"path":             "lore/creation",
		"title":            "creation",
		"expected_version": version,
		"sections":         []map[string]any{{"content": "v2"}},
	}
	if rec := env.do(t, http.MethodPatch, "/api/documents/"+docID, env.gmToken, update); rec.Code != http.StatusOK {
		t.Fatalf("first update status = %d body %s", rec.Code, rec.Body.String())
	}

	// Same stale version again: warn.
	stale := env.do(t, http.MethodPatch, "/api/documents/"+docID, env.gmToken, update)
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale update status = %d, want 409", stale.Code)
	}
	if !strings.Contains(stale.Body.String(), `"code":"concurrent_edit"`) {
		t.Fatalf("conflict body missing code: %s", stale.Body.String())
	}

	update["force"] = true
	if rec := env.do(t, http.MethodPatch, "/api/documents/"+docID, env.gmToken, update); rec.Code != http.StatusOK {
		t.Fatalf("forced update status = %d body %s", rec.Code, rec.Body.String())
	}
}

func TestDocumentRoutes_ShareToGroup(t *testing.T) {
	env := newDocumentsTransportEnv(t)
	ctx := t.Context()

	group, err := env.groups.Create(ctx, env.gmRequester, "Thieves Guild", models.GroupTypeOrganization, "")
	require.NoError(t, err)
	err = env.groups.Join(ctx, env.gmRequester, group.ID, env.characterID)
	require.NoError(t, err)

	charRepoID := env.findRepositoryID(t, models.RepositoryTypeCharacter, env.characterID)
	groupRepoID := env.findRepositoryID(t, models.RepositoryTypeGroup, group.ID)

	rec := env.do(t, http.MethodPost, "/api/repositories/"+charRepoID+"/documents", env.playerToken,
		map[string]any{"path": "notes/suspicions"})
	require.Equal(t, rec.Code, http.StatusCreated)

	var doc map[string]any
	err = json.Unmarshal(rec.Body.Bytes(), &doc)
	require.NoError(t, err)
	docID, _ := doc["id"].(string)

	share := env.do(t, http.MethodPost, "/api/documents/"+docID+"/share", env.playerToken,
		map[string]any{"target_repository_id": groupRepoID, "shared_on_game_day": 2})
	if share.Code != http.StatusOK {
		t.Fatalf("share status = %d, want 200: %s", share.Code, share.Body.String())
	}

	var shared map[string]any
	err = json.Unmarshal(share.Body.Bytes(), &shared)
	require.NoError(t, err)
	if shared["repository_id"] != groupRepoID {
		t.Fatalf("repository_id = %v, want %q", shared["repository_id"], groupRepoID)
	}

	// It no longer lists under the character repository.
	list := env.do(t, http.MethodGet, "/api/repositories/"+charRepoID+"/documents", env.playerToken, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list character repo: status %d", list.Code)
	}
	if strings.Contains(list.Body.String(), "suspicions") {
		t.Fatalf("shared document still lists under the character repository: %s", list.Body.String())
	}
}

func TestDocumentRoutes_ShareToGroup_NonMemberGets404(t *testing.T) {
	env := newDocumentsTransportEnv(t)
	ctx := t.Context()

	group, err := env.groups.Create(ctx, env.gmRequester, "Thieves Guild", models.GroupTypeOrganization, "")
	require.NoError(t, err)
	// Note: the character never joins the group.

	charRepoID := env.findRepositoryID(t, models.RepositoryTypeCharacter, env.characterID)
	groupRepoID := env.findRepositoryID(t, models.RepositoryTypeGroup, group.ID)

	rec := env.do(t, http.MethodPost, "/api/repositories/"+charRepoID+"/documents", env.playerToken,
		map[string]any{"path": "notes/suspicions"})
	require.Equal(t, rec.Code, http.StatusCreated)

	var doc map[string]any
	err = json.Unmarshal(rec.Body.Bytes(), &doc)
	require.NoError(t, err)
	docID, _ := doc["id"].(string)

	share := env.do(t, http.MethodPost, "/api/documents/"+docID+"/share", env.playerToken,
		map[string]any{"target_repository_id": groupRepoID, "shared_on_game_day": 2})
	require.Equal(t, share.Code, http.StatusNotFound)
}

func TestDocumentRoutes_DirectShare_GatesByCharacterGameDay(t *testing.T) {
	env := newDocumentsTransportEnv(t)

	// A character-only document, private to some other character, so the
	// player's own repository access never grants it.
	otherID := env.createOtherCharacterRepository(t)
	doc := env.createDocumentIn(t, otherID, map[string]any{
		"path":     "secrets/heirloom",
		"sections": []map[string]any{{"content": "The ring is cursed."}},
	})
	docID, _ := doc["id"].(string)

	// Not shared yet: hidden.
	if rec := env.do(t, http.MethodGet, "/api/documents/"+docID, env.playerToken, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("pre-share GET status = %d, want 404", rec.Code)
	}

	share := env.do(t, http.MethodPost, "/api/documents/"+docID+"/shares", env.gmToken,
		map[string]any{"character_id": env.characterID, "shared_on_game_day": 3})
	if share.Code != http.StatusCreated {
		t.Fatalf("share status = %d body %s", share.Code, share.Body.String())
	}

	// Shared, but game day not yet reached: still hidden.
	if rec := env.do(t, http.MethodGet, "/api/documents/"+docID, env.playerToken, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("pre-day GET status = %d, want 404", rec.Code)
	}

	env.setCharacterGameDay(t, 3)

	// Game day reached: visible, and GM-only sections still stripped.
	rec := env.do(t, http.MethodGet, "/api/documents/"+docID, env.playerToken, nil)
	require.Equal(t, rec.Code, http.StatusOK)

	if !strings.Contains(rec.Body.String(), "cursed") {
		t.Fatalf("shared document content missing: %s", rec.Body.String())
	}

	// A GM can list and revoke the share.
	list := env.do(t, http.MethodGet, "/api/documents/"+docID+"/shares", env.gmToken, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list shares status = %d body %s", list.Code, list.Body.String())
	}

	var shares []map[string]any
	err := json.Unmarshal(list.Body.Bytes(), &shares)
	require.NoError(t, err)
	if len(shares) != 1 {
		t.Fatalf("shares = %+v, want 1", shares)
	}
	shareID, _ := shares[0]["id"].(string)

	revoke := env.do(t, http.MethodDelete, "/api/documents/"+docID+"/shares/"+shareID, env.gmToken, nil)
	if revoke.Code != http.StatusNoContent {
		t.Fatalf("revoke status = %d body %s", revoke.Code, revoke.Body.String())
	}

	if rec := env.do(t, http.MethodGet, "/api/documents/"+docID, env.playerToken, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("post-revoke GET status = %d, want 404", rec.Code)
	}
}

func TestDocumentRoutes_ShareRoutes_ForbiddenForPlayers(t *testing.T) {
	env := newDocumentsTransportEnv(t)

	doc := env.createDocument(t, map[string]any{"path": "lore/creation"})
	docID, _ := doc["id"].(string)

	share := env.do(t, http.MethodPost, "/api/documents/"+docID+"/shares", env.playerToken,
		map[string]any{"character_id": env.characterID, "shared_on_game_day": 1})
	if share.Code != http.StatusForbidden {
		t.Fatalf("player share status = %d, want 403", share.Code)
	}
}

func TestDocumentRoutes_ListSharedWithMe(t *testing.T) {
	env := newDocumentsTransportEnv(t)

	otherID := env.createOtherCharacterRepository(t)
	doc := env.createDocumentIn(t, otherID, map[string]any{"path": "secrets/heirloom"})
	docID, _ := doc["id"].(string)

	if rec := env.do(t, http.MethodPost, "/api/documents/"+docID+"/shares", env.gmToken,
		map[string]any{"character_id": env.characterID, "shared_on_game_day": 0}); rec.Code != http.StatusCreated {
		t.Fatalf("share status = %d body %s", rec.Code, rec.Body.String())
	}

	rec := env.do(t, http.MethodGet, "/api/documents/shared", env.playerToken, nil)
	require.Equal(t, rec.Code, http.StatusOK)

	var docs []map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &docs)
	require.NoError(t, err)
	if len(docs) != 1 || docs[0]["id"] != docID {
		t.Fatalf("shared docs = %+v, want [%s]", docs, docID)
	}
}
