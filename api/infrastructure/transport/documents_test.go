package transport_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
	"github.com/stretchr/testify/assert"
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

	require.Failf(t, "repository not found", "no %s repository found for owner %q", repoType, ownerID)

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
	require.Equal(t, http.StatusCreated, rec.Code)

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
		require.NoError(t, err)

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
	require.Equal(t, http.StatusCreated, rec.Code)

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
	require.Equal(t, http.StatusOK, rec.Code)

	// Grep the raw payload, not the parsed struct: nothing GM-only may be in
	// the bytes a player receives.
	payload := rec.Body.String()
	assert.NotContains(t, payload, "vampire", "GM-only content leaked into player payload")
	assert.NotContains(t, payload, `"gm_only":true`, "GM-only content leaked into player payload")
}

func TestDocumentRoutes_GameDayGate_Returns404NotForbidden(t *testing.T) {
	env := newDocumentsTransportEnv(t)

	doc := env.createDocument(t, map[string]any{
		"path":               "reveals/the-betrayal",
		"shared_on_game_day": 5,
	})

	docID, _ := doc["id"].(string)
	rec := env.do(t, http.MethodGet, "/api/documents/"+docID, env.playerToken, nil)
	require.Equal(t, http.StatusNotFound, rec.Code)

	list := env.do(t, http.MethodGet, "/api/repositories/"+env.generalID+"/documents", env.playerToken, nil)
	require.Equal(t, http.StatusOK, list.Code)
	assert.NotContains(t, list.Body.String(), "the-betrayal", "gated document leaked into list")
}

func TestDocumentRoutes_FolderTree_HidesGatedFolder(t *testing.T) {
	env := newDocumentsTransportEnv(t)

	env.createDocument(t, map[string]any{"path": "lore/creation"})
	env.createDocument(t, map[string]any{
		"path":               "secrets/the-betrayal",
		"shared_on_game_day": 5,
	})

	rec := env.do(t, http.MethodGet, "/api/repositories/"+env.generalID+"/documents/tree", env.playerToken, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var tree struct {
		Folders []struct {
			Name string `json:"name"`
		} `json:"folders"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &tree)
	require.NoError(t, err)

	if assert.Len(t, tree.Folders, 1, "the unrevealed secrets folder must not leak") {
		assert.Equal(t, "lore", tree.Folders[0].Name)
	}
}

func TestDocumentRoutes_PathCollision_409WithCode(t *testing.T) {
	env := newDocumentsTransportEnv(t)

	env.createDocument(t, map[string]any{"path": "lore/creation"})

	rec := env.do(t, http.MethodPost, "/api/repositories/"+env.generalID+"/documents", env.gmToken,
		map[string]any{"path": "lore/creation"})
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"path_collision"`)

	allowed := env.do(t, http.MethodPost, "/api/repositories/"+env.generalID+"/documents", env.gmToken,
		map[string]any{"path": "lore/creation", "allow_collision": true})
	assert.Equal(t, http.StatusCreated, allowed.Code)
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
	rec := env.do(t, http.MethodPatch, "/api/documents/"+docID, env.gmToken, update)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Same stale version again: warn.
	stale := env.do(t, http.MethodPatch, "/api/documents/"+docID, env.gmToken, update)
	require.Equal(t, http.StatusConflict, stale.Code)
	assert.Contains(t, stale.Body.String(), `"code":"concurrent_edit"`)

	update["force"] = true
	rec = env.do(t, http.MethodPatch, "/api/documents/"+docID, env.gmToken, update)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
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
	require.Equal(t, http.StatusCreated, rec.Code)

	var doc map[string]any
	err = json.Unmarshal(rec.Body.Bytes(), &doc)
	require.NoError(t, err)
	docID, _ := doc["id"].(string)

	share := env.do(t, http.MethodPost, "/api/documents/"+docID+"/share", env.playerToken,
		map[string]any{"target_repository_id": groupRepoID, "shared_on_game_day": 2})
	require.Equal(t, http.StatusOK, share.Code, share.Body.String())

	var shared map[string]any
	err = json.Unmarshal(share.Body.Bytes(), &shared)
	require.NoError(t, err)
	assert.Equal(t, shared["repository_id"], groupRepoID)

	// It no longer lists under the character repository.
	list := env.do(t, http.MethodGet, "/api/repositories/"+charRepoID+"/documents", env.playerToken, nil)
	require.Equal(t, http.StatusOK, list.Code)
	assert.NotContains(t, list.Body.String(), "suspicions", "shared document still lists under the character repository")
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
	require.Equal(t, http.StatusCreated, rec.Code)

	var doc map[string]any
	err = json.Unmarshal(rec.Body.Bytes(), &doc)
	require.NoError(t, err)
	docID, _ := doc["id"].(string)

	share := env.do(t, http.MethodPost, "/api/documents/"+docID+"/share", env.playerToken,
		map[string]any{"target_repository_id": groupRepoID, "shared_on_game_day": 2})
	require.Equal(t, http.StatusNotFound, share.Code)
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
	preShare := env.do(t, http.MethodGet, "/api/documents/"+docID, env.playerToken, nil)
	require.Equal(t, http.StatusNotFound, preShare.Code)

	share := env.do(t, http.MethodPost, "/api/documents/"+docID+"/shares", env.gmToken,
		map[string]any{"character_id": env.characterID, "shared_on_game_day": 3})
	require.Equal(t, http.StatusCreated, share.Code, share.Body.String())

	// Shared, but game day not yet reached: still hidden.
	preDay := env.do(t, http.MethodGet, "/api/documents/"+docID, env.playerToken, nil)
	require.Equal(t, http.StatusNotFound, preDay.Code)

	env.setCharacterGameDay(t, 3)

	// Game day reached: visible, and GM-only sections still stripped.
	rec := env.do(t, http.MethodGet, "/api/documents/"+docID, env.playerToken, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	assert.Contains(t, rec.Body.String(), "cursed", "shared document content missing")

	// A GM can list and revoke the share.
	list := env.do(t, http.MethodGet, "/api/documents/"+docID+"/shares", env.gmToken, nil)
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())

	var shares []map[string]any
	err := json.Unmarshal(list.Body.Bytes(), &shares)
	require.NoError(t, err)
	require.Len(t, shares, 1)
	shareID, _ := shares[0]["id"].(string)

	revoke := env.do(t, http.MethodDelete, "/api/documents/"+docID+"/shares/"+shareID, env.gmToken, nil)
	require.Equal(t, http.StatusNoContent, revoke.Code, revoke.Body.String())

	postRevoke := env.do(t, http.MethodGet, "/api/documents/"+docID, env.playerToken, nil)
	assert.Equal(t, http.StatusNotFound, postRevoke.Code)
}

func TestDocumentRoutes_ShareRoutes_ForbiddenForPlayers(t *testing.T) {
	env := newDocumentsTransportEnv(t)

	doc := env.createDocument(t, map[string]any{"path": "lore/creation"})
	docID, _ := doc["id"].(string)

	share := env.do(t, http.MethodPost, "/api/documents/"+docID+"/shares", env.playerToken,
		map[string]any{"character_id": env.characterID, "shared_on_game_day": 1})
	assert.Equal(t, http.StatusForbidden, share.Code)
}

func TestDocumentRoutes_ListSharedWithMe(t *testing.T) {
	env := newDocumentsTransportEnv(t)

	otherID := env.createOtherCharacterRepository(t)
	doc := env.createDocumentIn(t, otherID, map[string]any{"path": "secrets/heirloom"})
	docID, _ := doc["id"].(string)

	shareRec := env.do(t, http.MethodPost, "/api/documents/"+docID+"/shares", env.gmToken,
		map[string]any{"character_id": env.characterID, "shared_on_game_day": 0})
	require.Equal(t, http.StatusCreated, shareRec.Code, shareRec.Body.String())

	rec := env.do(t, http.MethodGet, "/api/documents/shared", env.playerToken, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var docs []map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &docs)
	require.NoError(t, err)
	if assert.Len(t, docs, 1) {
		assert.Equal(t, docs[0]["id"], docID)
	}
}
