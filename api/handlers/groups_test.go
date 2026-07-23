package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/handlers"
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/DaanV2/itinerarium/api/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type groupTestEnv struct {
	router      *transport.Router
	gmToken     string
	playerToken string
	otherToken  string
	characterID string
}

func newGroupTestEnv(t *testing.T) groupTestEnv {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err, "persistence.New")
	require.NoError(t, db.Migrate(), "Migrate")

	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(t.TempDir()))
	require.NoError(t, err, "NewKeyStore")

	tokens := authentication.NewTokenService(keys, repositories.NewRevokedTokens(db))
	users := repositories.NewUsers(db)
	characters := repositories.NewCharacters(db)
	authSvc := application.NewAuthService(tokens, users)
	knowledgeRepo := repositories.NewKnowledgeRepositories(db)
	characterSvc := application.NewCharacterService(characters, users, knowledgeRepo)
	groupSvc := application.NewGroupService(repositories.NewGroups(db), characterSvc, knowledgeRepo)
	requireAuth := transport.RequireAuth(authSvc)

	ctx := t.Context()
	gm := &models.User{Email: "gm@example.com", PasswordHash: "hash", Role: models.RoleGM}
	player := &models.User{Email: "player@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	other := &models.User{Email: "other@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	for _, u := range []*models.User{gm, player, other} {
		require.NoError(t, users.Create(ctx, u), "Create user")
	}

	character := &models.Character{Name: "Aria", UserID: player.ID}
	require.NoError(t, characters.Create(ctx, character), "Create character")

	router := transport.NewRouter(
		transport.WithHandle("GET /api/groups", requireAuth(handlers.ListGroupsHandler(groupSvc))),
		transport.WithHandle("POST /api/groups", requireAuth(handlers.CreateGroupHandler(groupSvc))),
		transport.WithHandle("GET /api/groups/{id}", requireAuth(handlers.GetGroupHandler(groupSvc))),
		transport.WithHandle("PATCH /api/groups/{id}", requireAuth(handlers.UpdateGroupHandler(groupSvc))),
		transport.WithHandle("POST /api/groups/{id}/members", requireAuth(handlers.JoinGroupHandler(groupSvc))),
		transport.WithHandle(
			"DELETE /api/groups/{id}/members/{characterId}", requireAuth(handlers.LeaveGroupHandler(groupSvc)),
		),
	)

	return groupTestEnv{
		router:      router,
		gmToken:     issueToken(t, tokens, gm.ID),
		playerToken: issueToken(t, tokens, player.ID),
		otherToken:  issueToken(t, tokens, other.ID),
		characterID: character.ID,
	}
}

func (e groupTestEnv) doJSON(t *testing.T, method, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	if payload != nil {
		require.NoError(t, json.NewEncoder(&body).Encode(payload), "encoding request")
	}

	req := httptest.NewRequestWithContext(t.Context(), method, path, &body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)

	return rec
}

func (e groupTestEnv) createGroup(t *testing.T, name string) string {
	t.Helper()

	rec := e.doJSON(t, http.MethodPost, "/api/groups", e.gmToken,
		map[string]any{"name": name, "type": "organization"})
	require.Equal(t, http.StatusCreated, rec.Code, "create group body: %s", rec.Body.String())

	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created), "decoding group")

	return created.ID
}

func TestGroups_RequireAuth(t *testing.T) {
	env := newGroupTestEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/groups", "", nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code, "body: %s", rec.Body.String())
}

func TestGroups_CreateIsGMOnly(t *testing.T) {
	env := newGroupTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/groups", env.playerToken,
		map[string]any{"name": "Thieves Guild", "type": "organization"})
	require.Equal(t, http.StatusForbidden, rec.Code, "player create body: %s", rec.Body.String())
}

func TestGroups_JoinAndLeaveOwnCharacter(t *testing.T) {
	env := newGroupTestEnv(t)
	groupID := env.createGroup(t, "Thieves Guild")

	joinRec := env.doJSON(t, http.MethodPost, "/api/groups/"+groupID+"/members", env.playerToken,
		map[string]any{"character_id": env.characterID})
	require.Equal(t, http.StatusNoContent, joinRec.Code, "join body: %s", joinRec.Body.String())

	getRec := env.doJSON(t, http.MethodGet, "/api/groups/"+groupID, env.playerToken, nil)
	require.Equal(t, http.StatusOK, getRec.Code, "get body: %s", getRec.Body.String())

	var group struct {
		Members []struct {
			ID string `json:"id"`
		} `json:"members"`
	}
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &group), "decoding group")
	require.Len(t, group.Members, 1)
	require.Equal(t, env.characterID, group.Members[0].ID)

	leaveRec := env.doJSON(
		t, http.MethodDelete, "/api/groups/"+groupID+"/members/"+env.characterID, env.playerToken, nil,
	)
	require.Equal(t, http.StatusNoContent, leaveRec.Code, "leave body: %s", leaveRec.Body.String())
}

func TestGroups_JoinWithForeignCharacterIs404(t *testing.T) {
	env := newGroupTestEnv(t)
	groupID := env.createGroup(t, "Thieves Guild")

	// Another player using someone else's character must get 404 — proving the
	// character exists would be a leak (hidden means invisible).
	rec := env.doJSON(t, http.MethodPost, "/api/groups/"+groupID+"/members", env.otherToken,
		map[string]any{"character_id": env.characterID})
	require.Equal(t, http.StatusNotFound, rec.Code, "foreign character body: %s", rec.Body.String())
}

func TestGroups_MemberResponseExposesOnlyIdentity(t *testing.T) {
	env := newGroupTestEnv(t)
	groupID := env.createGroup(t, "Thieves Guild")

	joinRec := env.doJSON(t, http.MethodPost, "/api/groups/"+groupID+"/members", env.playerToken,
		map[string]any{"character_id": env.characterID})
	require.Equal(t, http.StatusNoContent, joinRec.Code, "join body: %s", joinRec.Body.String())

	// Other players can see who is in a group, but not the members' game days
	// or owning accounts.
	getRec := env.doJSON(t, http.MethodGet, "/api/groups/"+groupID, env.otherToken, nil)
	require.Equal(t, http.StatusOK, getRec.Code, "get body: %s", getRec.Body.String())

	var group struct {
		Members []map[string]any `json:"members"`
	}
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &group), "decoding group")
	require.Len(t, group.Members, 1)

	for _, forbidden := range []string{"current_game_day", "user_id"} {
		assert.NotContains(t, group.Members[0], forbidden, "member response leaks %q", forbidden)
	}
}
