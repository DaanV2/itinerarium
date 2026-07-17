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

type charactersTestEnv struct {
	router      *transport.Router
	gmToken     string
	playerToken string
	otherToken  string
}

func newCharactersTestEnv(t *testing.T) charactersTestEnv {
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
	authSvc := application.NewAuthService(tokens, users)
	characterSvc := application.NewCharacterService(characters, users, repositories.NewKnowledgeRepositories(db))
	requireAuth := transport.RequireAuth(authSvc)

	ctx := t.Context()

	gm := &models.User{Email: "gm@example.com", PasswordHash: "hash", Role: models.RoleGM}
	err = users.Create(ctx, gm)
	require.NoError(t, err)

	player := &models.User{Email: "player@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	err = users.Create(ctx, player)
	require.NoError(t, err)

	other := &models.User{Email: "other@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	err = users.Create(ctx, other)
	require.NoError(t, err)

	gmToken, err := tokens.Issue(gm.ID)
	require.NoError(t, err)

	playerToken, err := tokens.Issue(player.ID)
	require.NoError(t, err)

	otherToken, err := tokens.Issue(other.ID)
	require.NoError(t, err)

	router := transport.NewRouter(
		transport.WithHandle("GET /api/characters", requireAuth(transport.ListCharactersHandler(characterSvc))),
		transport.WithHandle("POST /api/characters", requireAuth(transport.CreateCharacterHandler(characterSvc))),
		transport.WithHandle("GET /api/characters/{id}", requireAuth(transport.GetCharacterHandler(characterSvc))),
		transport.WithHandle(
			"PATCH /api/characters/{id}", requireAuth(transport.UpdateCharacterHandler(characterSvc)),
		),
	)

	return charactersTestEnv{router: router, gmToken: gmToken, playerToken: playerToken, otherToken: otherToken}
}

func (e charactersTestEnv) doJSON(t *testing.T, method, path, token string, payload any) *httptest.ResponseRecorder {
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

func TestCreateCharacter_RequiresAuth(t *testing.T) {
	env := newCharactersTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/characters", "", map[string]string{"name": "Aria"})
	require.Equal(t, rec.Code, http.StatusUnauthorized)
}

func TestCreateCharacter_ForSelf(t *testing.T) {
	env := newCharactersTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/characters", env.playerToken, map[string]string{"name": "Aria"})
	require.Equal(t, rec.Code, http.StatusCreated)

	var created struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		CurrentGameDay int    `json:"current_game_day"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &created)
	require.NoError(t, err)
	if created.Name != "Aria" {
		t.Fatalf("Name = %q, want Aria", created.Name)
	}
	if created.CurrentGameDay != 0 {
		t.Fatalf("CurrentGameDay = %d, want 0", created.CurrentGameDay)
	}
}

func TestCreateCharacter_AllowsMultiplePerUser(t *testing.T) {
	env := newCharactersTestEnv(t)

	first := env.doJSON(t, http.MethodPost, "/api/characters", env.playerToken, map[string]string{"name": "Aria"})
	if first.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", first.Code, first.Body.String())
	}

	second := env.doJSON(t, http.MethodPost, "/api/characters", env.playerToken, map[string]string{"name": "Beren"})
	if second.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", second.Code, second.Body.String())
	}

	listRec := env.doJSON(t, http.MethodGet, "/api/characters", env.playerToken, nil)

	var list []struct{ Name string }
	err := json.Unmarshal(listRec.Body.Bytes(), &list)
	require.NoError(t, err)
	if len(list) != 2 {
		t.Fatalf("expected 2 characters, got %d", len(list))
	}
}

func TestListCharacters_PlayerSeesOnlyOwn(t *testing.T) {
	env := newCharactersTestEnv(t)

	env.doJSON(t, http.MethodPost, "/api/characters", env.playerToken, map[string]string{"name": "Aria"})
	env.doJSON(t, http.MethodPost, "/api/characters", env.otherToken, map[string]string{"name": "Beren"})

	rec := env.doJSON(t, http.MethodGet, "/api/characters", env.playerToken, nil)

	var list []struct{ Name string }
	err := json.Unmarshal(rec.Body.Bytes(), &list)
	require.NoError(t, err)
	if len(list) != 1 {
		t.Fatalf("expected 1 character, got %d", len(list))
	}
}

func TestGetCharacter_HidesOtherOwnersCharacter(t *testing.T) {
	env := newCharactersTestEnv(t)

	createRec := env.doJSON(t, http.MethodPost, "/api/characters", env.otherToken, map[string]string{"name": "Beren"})

	var created struct{ ID string }
	err := json.Unmarshal(createRec.Body.Bytes(), &created)
	require.NoError(t, err)

	rec := env.doJSON(t, http.MethodGet, "/api/characters/"+created.ID, env.playerToken, nil)
	require.Equal(t, rec.Code, http.StatusNotFound)
}

func TestUpdateCharacter_PlayerCannotSetGameDay(t *testing.T) {
	env := newCharactersTestEnv(t)

	createRec := env.doJSON(t, http.MethodPost, "/api/characters", env.playerToken, map[string]string{"name": "Aria"})

	var created struct{ ID string }
	err := json.Unmarshal(createRec.Body.Bytes(), &created)
	require.NoError(t, err)

	rec := env.doJSON(t, http.MethodPatch, "/api/characters/"+created.ID, env.playerToken,
		map[string]int{"current_game_day": 5})
	require.Equal(t, rec.Code, http.StatusForbidden)
}

func TestUpdateCharacter_GMCanSetGameDay(t *testing.T) {
	env := newCharactersTestEnv(t)

	createRec := env.doJSON(t, http.MethodPost, "/api/characters", env.playerToken, map[string]string{"name": "Aria"})

	var created struct{ ID string }
	err := json.Unmarshal(createRec.Body.Bytes(), &created)
	require.NoError(t, err)

	rec := env.doJSON(t, http.MethodPatch, "/api/characters/"+created.ID, env.gmToken,
		map[string]int{"current_game_day": 5})
	require.Equal(t, rec.Code, http.StatusOK)

	var updated struct {
		CurrentGameDay int `json:"current_game_day"`
	}
	err = json.Unmarshal(rec.Body.Bytes(), &updated)
	require.NoError(t, err)
	if updated.CurrentGameDay != 5 {
		t.Fatalf("CurrentGameDay = %d, want 5", updated.CurrentGameDay)
	}
}
