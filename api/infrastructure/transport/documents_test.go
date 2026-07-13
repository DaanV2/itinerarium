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
)

type documentsTransportEnv struct {
	router      *transport.Router
	gmToken     string
	playerToken string
	generalID   string
}

func newDocumentsTransportEnv(t *testing.T) documentsTransportEnv {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	if err != nil {
		t.Fatalf("persistence.New: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(t.TempDir()))
	if err != nil {
		t.Fatalf("NewKeyStore: %v", err)
	}

	tokens := authentication.NewTokenService(keys, repositories.NewRevokedTokens(db))
	users := repositories.NewUsers(db)
	characters := repositories.NewCharacters(db)
	groups := repositories.NewGroups(db)
	knowledgeRepos := repositories.NewKnowledgeRepositories(db)

	authSvc := application.NewAuthService(tokens, users)
	repoSvc := application.NewRepositoryService(knowledgeRepos, groups, characters)
	docSvc := application.NewDocumentService(repositories.NewDocuments(db), repoSvc, characters, groups)
	requireAuth := transport.RequireAuth(authSvc)

	ctx := t.Context()

	if err := repoSvc.EnsureSystemRepositories(ctx); err != nil {
		t.Fatalf("EnsureSystemRepositories: %v", err)
	}

	general, err := knowledgeRepos.EnsureGeneral(ctx)
	if err != nil {
		t.Fatalf("EnsureGeneral: %v", err)
	}

	gm := &models.User{Email: "gm@example.com", PasswordHash: "hash", Role: models.RoleGM}
	if err := users.Create(ctx, gm); err != nil {
		t.Fatalf("Create gm: %v", err)
	}

	player := &models.User{Email: "player@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	if err := users.Create(ctx, player); err != nil {
		t.Fatalf("Create player: %v", err)
	}

	gmToken, err := tokens.Issue(gm.ID)
	if err != nil {
		t.Fatalf("Issue(gm): %v", err)
	}

	playerToken, err := tokens.Issue(player.ID)
	if err != nil {
		t.Fatalf("Issue(player): %v", err)
	}

	// The player needs a character: game-day gating resolves through it.
	if err := characters.Create(ctx, &models.Character{Name: "Aria", UserID: player.ID}); err != nil {
		t.Fatalf("Create character: %v", err)
	}

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
		transport.WithHandle("GET /api/documents/{id}", requireAuth(transport.GetDocumentHandler(docSvc))),
		transport.WithHandle("PATCH /api/documents/{id}", requireAuth(transport.UpdateDocumentHandler(docSvc))),
	)

	return documentsTransportEnv{
		router:      router,
		gmToken:     gmToken,
		playerToken: playerToken,
		generalID:   general.ID,
	}
}

func (e documentsTransportEnv) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
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

func (e documentsTransportEnv) createDocument(t *testing.T, body map[string]any) map[string]any {
	t.Helper()

	rec := e.do(t, http.MethodPost, "/api/repositories/"+e.generalID+"/documents", e.gmToken, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create document: status %d body %s", rec.Code, rec.Body.String())
	}

	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

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
	if rec.Code != http.StatusOK {
		t.Fatalf("player GET status = %d, want 200", rec.Code)
	}

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
	if rec.Code != http.StatusNotFound {
		t.Fatalf("gated GET status = %d, want 404 (never 403)", rec.Code)
	}

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
	if rec.Code != http.StatusOK {
		t.Fatalf("tree GET status = %d body %s", rec.Code, rec.Body.String())
	}

	var tree struct {
		Folders []struct {
			Name string `json:"name"`
		} `json:"folders"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tree); err != nil {
		t.Fatalf("decode tree: %v", err)
	}

	if len(tree.Folders) != 1 || tree.Folders[0].Name != "lore" {
		t.Fatalf("folders = %+v, want [lore] — the unrevealed secrets folder must not leak", tree.Folders)
	}
}

func TestDocumentRoutes_PathCollision_409WithCode(t *testing.T) {
	env := newDocumentsTransportEnv(t)

	env.createDocument(t, map[string]any{"path": "lore/creation"})

	rec := env.do(t, http.MethodPost, "/api/repositories/"+env.generalID+"/documents", env.gmToken,
		map[string]any{"path": "lore/creation"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("colliding create status = %d, want 409", rec.Code)
	}
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
