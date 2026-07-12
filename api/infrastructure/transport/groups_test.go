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
		if err := users.Create(ctx, u); err != nil {
			t.Fatalf("Create user: %v", err)
		}
	}

	character := &models.Character{Name: "Aria", UserID: player.ID}
	if err := characters.Create(ctx, character); err != nil {
		t.Fatalf("Create character: %v", err)
	}

	router := transport.NewRouter(
		transport.WithHandle("GET /api/groups", requireAuth(transport.ListGroupsHandler(groupSvc))),
		transport.WithHandle("POST /api/groups", requireAuth(transport.CreateGroupHandler(groupSvc))),
		transport.WithHandle("GET /api/groups/{id}", requireAuth(transport.GetGroupHandler(groupSvc))),
		transport.WithHandle("PATCH /api/groups/{id}", requireAuth(transport.UpdateGroupHandler(groupSvc))),
		transport.WithHandle("POST /api/groups/{id}/members", requireAuth(transport.JoinGroupHandler(groupSvc))),
		transport.WithHandle(
			"DELETE /api/groups/{id}/members/{characterId}", requireAuth(transport.LeaveGroupHandler(groupSvc)),
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
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatalf("encoding request: %v", err)
		}
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
	if rec.Code != http.StatusCreated {
		t.Fatalf("create group: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding group: %v", err)
	}

	return created.ID
}

func TestGroups_RequireAuth(t *testing.T) {
	env := newGroupTestEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/groups", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no token, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGroups_CreateIsGMOnly(t *testing.T) {
	env := newGroupTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/groups", env.playerToken,
		map[string]any{"name": "Thieves Guild", "type": "organization"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for player create, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGroups_JoinAndLeaveOwnCharacter(t *testing.T) {
	env := newGroupTestEnv(t)
	groupID := env.createGroup(t, "Thieves Guild")

	joinRec := env.doJSON(t, http.MethodPost, "/api/groups/"+groupID+"/members", env.playerToken,
		map[string]any{"character_id": env.characterID})
	if joinRec.Code != http.StatusNoContent {
		t.Fatalf("join: expected 204, got %d: %s", joinRec.Code, joinRec.Body.String())
	}

	getRec := env.doJSON(t, http.MethodGet, "/api/groups/"+groupID, env.playerToken, nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var group struct {
		Members []struct {
			ID string `json:"id"`
		} `json:"members"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &group); err != nil {
		t.Fatalf("decoding group: %v", err)
	}
	if len(group.Members) != 1 || group.Members[0].ID != env.characterID {
		t.Fatalf("members = %v, want [%s]", group.Members, env.characterID)
	}

	leaveRec := env.doJSON(
		t, http.MethodDelete, "/api/groups/"+groupID+"/members/"+env.characterID, env.playerToken, nil,
	)
	if leaveRec.Code != http.StatusNoContent {
		t.Fatalf("leave: expected 204, got %d: %s", leaveRec.Code, leaveRec.Body.String())
	}
}

func TestGroups_JoinWithForeignCharacterIs404(t *testing.T) {
	env := newGroupTestEnv(t)
	groupID := env.createGroup(t, "Thieves Guild")

	// Another player using someone else's character must get 404 — proving the
	// character exists would be a leak (hidden means invisible).
	rec := env.doJSON(t, http.MethodPost, "/api/groups/"+groupID+"/members", env.otherToken,
		map[string]any{"character_id": env.characterID})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for foreign character, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGroups_MemberResponseExposesOnlyIdentity(t *testing.T) {
	env := newGroupTestEnv(t)
	groupID := env.createGroup(t, "Thieves Guild")

	joinRec := env.doJSON(t, http.MethodPost, "/api/groups/"+groupID+"/members", env.playerToken,
		map[string]any{"character_id": env.characterID})
	if joinRec.Code != http.StatusNoContent {
		t.Fatalf("join: expected 204, got %d: %s", joinRec.Code, joinRec.Body.String())
	}

	// Other players can see who is in a group, but not the members' game days
	// or owning accounts.
	getRec := env.doJSON(t, http.MethodGet, "/api/groups/"+groupID, env.otherToken, nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var group struct {
		Members []map[string]any `json:"members"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &group); err != nil {
		t.Fatalf("decoding group: %v", err)
	}
	if len(group.Members) != 1 {
		t.Fatalf("members = %v, want exactly one", group.Members)
	}
	for _, forbidden := range []string{"current_game_day", "user_id"} {
		if _, ok := group.Members[0][forbidden]; ok {
			t.Errorf("member response leaks %q: %v", forbidden, group.Members[0])
		}
	}
}
