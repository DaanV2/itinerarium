package transport_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
)

type repositoriesTestEnv struct {
	router      *transport.Router
	gmToken     string
	playerToken string
	otherToken  string
	characterID string
}

func newRepositoriesTestEnv(t *testing.T) repositoriesTestEnv {
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
	knowledgeRepo := repositories.NewKnowledgeRepositories(db)
	authSvc := application.NewAuthService(tokens, users)
	characterSvc := application.NewCharacterService(characters, users, knowledgeRepo)
	repoSvc := application.NewRepositoryService(knowledgeRepo, repositories.NewGroups(db), characters)
	requireAuth := transport.RequireAuth(authSvc)

	ctx := t.Context()
	if err := repoSvc.EnsureSystemRepositories(ctx); err != nil {
		t.Fatalf("EnsureSystemRepositories: %v", err)
	}

	gm := &models.User{Email: "gm@example.com", PasswordHash: "hash", Role: models.RoleGM}
	player := &models.User{Email: "player@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	other := &models.User{Email: "other@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	for _, u := range []*models.User{gm, player, other} {
		if err := users.Create(ctx, u); err != nil {
			t.Fatalf("Create user: %v", err)
		}
	}

	character, err := characterSvc.Create(ctx, application.UserRequester{User: player}, "", "Aria")
	if err != nil {
		t.Fatalf("Create character: %v", err)
	}

	router := transport.NewRouter(
		transport.WithHandle("GET /api/repositories", requireAuth(transport.ListRepositoriesHandler(repoSvc))),
		transport.WithHandle("GET /api/repositories/{id}", requireAuth(transport.GetRepositoryHandler(repoSvc))),
	)

	return repositoriesTestEnv{
		router:      router,
		gmToken:     issueToken(t, tokens, gm.ID),
		playerToken: issueToken(t, tokens, player.ID),
		otherToken:  issueToken(t, tokens, other.ID),
		characterID: character.ID,
	}
}

func (e repositoriesTestEnv) doJSON(t *testing.T, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, path, &bytes.Buffer{})
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)

	return rec
}

func TestRepositories_List_GMSeesGeneralTemplateAndCharacterRepository(t *testing.T) {
	env := newRepositoriesTestEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/repositories", env.gmToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got []struct {
		Type        string  `json:"type"`
		CharacterID *string `json:"character_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding body: %v", err)
	}

	// general + template + Aria's character repository.
	if len(got) != 3 {
		t.Fatalf("List returned %d repositories, want 3: %+v", len(got), got)
	}
}

func TestRepositories_List_OwnerSeesOwnCharacterRepository(t *testing.T) {
	env := newRepositoriesTestEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/repositories", env.playerToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got []struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("List returned %d repositories for owner, want 3: %+v", len(got), got)
	}
}

func TestRepositories_Get_ForeignCharacterRepositoryHidden(t *testing.T) {
	env := newRepositoriesTestEnv(t)

	// Find the character repository ID via the GM's list.
	listRec := env.doJSON(t, http.MethodGet, "/api/repositories", env.gmToken)
	var repos []struct {
		ID          string  `json:"id"`
		Type        string  `json:"type"`
		CharacterID *string `json:"character_id"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &repos); err != nil {
		t.Fatalf("decoding list: %v", err)
	}

	var repoID string
	for _, r := range repos {
		if r.Type == "character" && r.CharacterID != nil && *r.CharacterID == env.characterID {
			repoID = r.ID
		}
	}
	if repoID == "" {
		t.Fatalf("character repository not found in %+v", repos)
	}

	// Owner and GM can read it…
	if rec := env.doJSON(t, http.MethodGet, "/api/repositories/"+repoID, env.playerToken); rec.Code != http.StatusOK {
		t.Fatalf("owner Get: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec := env.doJSON(t, http.MethodGet, "/api/repositories/"+repoID, env.gmToken); rec.Code != http.StatusOK {
		t.Fatalf("GM Get: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// …a different player gets 404, never 403 (existence hidden).
	rec := env.doJSON(t, http.MethodGet, "/api/repositories/"+repoID, env.otherToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for foreign player, got %d: %s", rec.Code, rec.Body.String())
	}
}
