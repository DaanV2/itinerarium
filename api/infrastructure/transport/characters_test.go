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

type charactersTestEnv struct {
	router      *transport.Router
	gmToken     string
	playerToken string
	otherToken  string
}

func newCharactersTestEnv(t *testing.T) charactersTestEnv {
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
	characterSvc := application.NewCharacterService(characters, users)
	requireAuth := transport.RequireAuth(authSvc)

	ctx := t.Context()

	gm := &models.User{Email: "gm@example.com", PasswordHash: "hash", Role: models.RoleGM}
	if err := users.Create(ctx, gm); err != nil {
		t.Fatalf("Create gm: %v", err)
	}

	player := &models.User{Email: "player@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	if err := users.Create(ctx, player); err != nil {
		t.Fatalf("Create player: %v", err)
	}

	other := &models.User{Email: "other@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	if err := users.Create(ctx, other); err != nil {
		t.Fatalf("Create other: %v", err)
	}

	gmToken, err := tokens.Issue(gm.ID)
	if err != nil {
		t.Fatalf("Issue(gm): %v", err)
	}

	playerToken, err := tokens.Issue(player.ID)
	if err != nil {
		t.Fatalf("Issue(player): %v", err)
	}

	otherToken, err := tokens.Issue(other.ID)
	if err != nil {
		t.Fatalf("Issue(other): %v", err)
	}

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
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no token, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateCharacter_ForSelf(t *testing.T) {
	env := newCharactersTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/characters", env.playerToken, map[string]string{"name": "Aria"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		CurrentGameDay int    `json:"current_game_day"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
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
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
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
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 character, got %d", len(list))
	}
}

func TestGetCharacter_HidesOtherOwnersCharacter(t *testing.T) {
	env := newCharactersTestEnv(t)

	createRec := env.doJSON(t, http.MethodPost, "/api/characters", env.otherToken, map[string]string{"name": "Beren"})

	var created struct{ ID string }
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding body: %v", err)
	}

	rec := env.doJSON(t, http.MethodGet, "/api/characters/"+created.ID, env.playerToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateCharacter_PlayerCannotSetGameDay(t *testing.T) {
	env := newCharactersTestEnv(t)

	createRec := env.doJSON(t, http.MethodPost, "/api/characters", env.playerToken, map[string]string{"name": "Aria"})

	var created struct{ ID string }
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding body: %v", err)
	}

	rec := env.doJSON(t, http.MethodPatch, "/api/characters/"+created.ID, env.playerToken,
		map[string]int{"current_game_day": 5})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateCharacter_GMCanSetGameDay(t *testing.T) {
	env := newCharactersTestEnv(t)

	createRec := env.doJSON(t, http.MethodPost, "/api/characters", env.playerToken, map[string]string{"name": "Aria"})

	var created struct{ ID string }
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding body: %v", err)
	}

	rec := env.doJSON(t, http.MethodPatch, "/api/characters/"+created.ID, env.gmToken,
		map[string]int{"current_game_day": 5})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated struct {
		CurrentGameDay int `json:"current_game_day"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if updated.CurrentGameDay != 5 {
		t.Fatalf("CurrentGameDay = %d, want 5", updated.CurrentGameDay)
	}
}
