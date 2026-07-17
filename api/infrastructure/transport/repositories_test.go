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
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err, "persistence.New")
	require.NoError(t, db.Migrate(), "Migrate")

	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(t.TempDir()))
	require.NoError(t, err, "NewKeyStore")

	tokens := authentication.NewTokenService(keys, repositories.NewRevokedTokens(db))
	users := repositories.NewUsers(db)
	characters := repositories.NewCharacters(db)
	knowledgeRepo := repositories.NewKnowledgeRepositories(db)
	authSvc := application.NewAuthService(tokens, users)
	characterSvc := application.NewCharacterService(characters, users, knowledgeRepo)
	repoSvc := application.NewRepositoryService(knowledgeRepo, repositories.NewGroups(db), characters)
	requireAuth := transport.RequireAuth(authSvc)

	ctx := t.Context()
	require.NoError(t, repoSvc.EnsureSystemRepositories(ctx), "EnsureSystemRepositories")

	gm := &models.User{Email: "gm@example.com", PasswordHash: "hash", Role: models.RoleGM}
	player := &models.User{Email: "player@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	other := &models.User{Email: "other@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	for _, u := range []*models.User{gm, player, other} {
		require.NoError(t, users.Create(ctx, u), "Create user")
	}

	character, err := characterSvc.Create(ctx, application.UserRequester{User: player}, "", "Aria")
	require.NoError(t, err, "Create character")

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
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var got []struct {
		Type        string  `json:"type"`
		CharacterID *string `json:"character_id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got), "decoding body")

	// general + template + Aria's character repository.
	require.Len(t, got, 3)
}

func TestRepositories_List_OwnerSeesOwnCharacterRepository(t *testing.T) {
	env := newRepositoriesTestEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/repositories", env.playerToken)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var got []struct {
		Type string `json:"type"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got), "decoding body")
	require.Len(t, got, 3)
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
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &repos), "decoding list")

	var repoID string
	for _, r := range repos {
		if r.Type == "character" && r.CharacterID != nil && *r.CharacterID == env.characterID {
			repoID = r.ID
		}
	}

	require.NotEmpty(t, repoID, "character repository not found in %+v", repos)

	// Owner and GM can read it…
	rec := env.doJSON(t, http.MethodGet, "/api/repositories/"+repoID, env.playerToken)
	require.Equal(t, http.StatusOK, rec.Code, "owner Get body: %s", rec.Body.String())

	rec = env.doJSON(t, http.MethodGet, "/api/repositories/"+repoID, env.gmToken)
	require.Equal(t, http.StatusOK, rec.Code, "GM Get body: %s", rec.Body.String())

	// …a different player gets 404, never 403 (existence hidden).
	rec = env.doJSON(t, http.MethodGet, "/api/repositories/"+repoID, env.otherToken)
	require.Equal(t, http.StatusNotFound, rec.Code, "foreign player body: %s", rec.Body.String())
}
