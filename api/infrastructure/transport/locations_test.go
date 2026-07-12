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

type locationHTTPTestEnv struct {
	router      *transport.Router
	gmToken     string
	playerToken string
	characterID string
}

func newLocationHTTPTestEnv(t *testing.T) locationHTTPTestEnv {
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
	characterSvc := application.NewCharacterService(characters, users, repositories.NewKnowledgeRepositories(db))
	locationSvc := application.NewLocationService(
		repositories.NewLocations(db),
		repositories.NewLocationAccesses(db),
		repositories.NewGroups(db),
		characters,
		characterSvc,
	)
	requireAuth := transport.RequireAuth(authSvc)

	ctx := t.Context()
	gm := &models.User{Email: "gm@example.com", PasswordHash: "hash", Role: models.RoleGM}
	player := &models.User{Email: "player@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	for _, u := range []*models.User{gm, player} {
		if err := users.Create(ctx, u); err != nil {
			t.Fatalf("Create user: %v", err)
		}
	}

	character := &models.Character{Name: "Aria", UserID: player.ID}
	if err := characters.Create(ctx, character); err != nil {
		t.Fatalf("Create character: %v", err)
	}

	router := transport.NewRouter(
		transport.WithHandle("GET /api/locations", requireAuth(transport.ListLocationsHandler(locationSvc))),
		transport.WithHandle("POST /api/locations", requireAuth(transport.CreateLocationHandler(locationSvc))),
		transport.WithHandle("GET /api/locations/{id}", requireAuth(transport.GetLocationHandler(locationSvc))),
		transport.WithHandle("PATCH /api/locations/{id}", requireAuth(transport.UpdateLocationHandler(locationSvc))),
		transport.WithHandle(
			"POST /api/locations/{id}/access", requireAuth(transport.GrantLocationAccessHandler(locationSvc)),
		),
		transport.WithHandle(
			"GET /api/locations/{id}/access", requireAuth(transport.ListLocationAccessHandler(locationSvc)),
		),
		transport.WithHandle(
			"PUT /api/characters/{id}/location", requireAuth(transport.SetCharacterLocationHandler(locationSvc)),
		),
	)

	return locationHTTPTestEnv{
		router:      router,
		gmToken:     issueToken(t, tokens, gm.ID),
		playerToken: issueToken(t, tokens, player.ID),
		characterID: character.ID,
	}
}

func (e locationHTTPTestEnv) doJSON(
	t *testing.T, method, path, token string, payload any,
) *httptest.ResponseRecorder {
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

func (e locationHTTPTestEnv) createLocation(t *testing.T, name string) string {
	t.Helper()

	rec := e.doJSON(t, http.MethodPost, "/api/locations", e.gmToken,
		map[string]any{"name": name, "plane": "Material"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create location: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding location: %v", err)
	}

	return created.ID
}

func TestLocations_CreateIsGMOnly(t *testing.T) {
	env := newLocationHTTPTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/locations", env.playerToken,
		map[string]any{"name": "The Tavern"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for player create, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLocations_HiddenWithoutGrant(t *testing.T) {
	env := newLocationHTTPTestEnv(t)
	locationID := env.createLocation(t, "Hidden Vault")

	// Absent from the list…
	listRec := env.doJSON(t, http.MethodGet, "/api/locations", env.playerToken, nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}
	if body := listRec.Body.String(); body != "[]" && body != "null" {
		t.Fatalf("list leaked locations to player without grants: %s", body)
	}

	// …direct read is 404, not 403.
	getRec := env.doJSON(t, http.MethodGet, "/api/locations/"+locationID, env.playerToken, nil)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 without grant, got %d: %s", getRec.Code, getRec.Body.String())
	}

	// …and the access list is GM-only.
	accessRec := env.doJSON(t, http.MethodGet, "/api/locations/"+locationID+"/access", env.playerToken, nil)
	if accessRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for player access list, got %d: %s", accessRec.Code, accessRec.Body.String())
	}
}

func TestLocations_GrantThenVisibleAndAssignable(t *testing.T) {
	env := newLocationHTTPTestEnv(t)
	locationID := env.createLocation(t, "The Tavern")

	grantRec := env.doJSON(t, http.MethodPost, "/api/locations/"+locationID+"/access", env.gmToken,
		map[string]any{"character_id": env.characterID})
	if grantRec.Code != http.StatusCreated {
		t.Fatalf("grant: expected 201, got %d: %s", grantRec.Code, grantRec.Body.String())
	}

	getRec := env.doJSON(t, http.MethodGet, "/api/locations/"+locationID, env.playerToken, nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get with grant: expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}

	assignRec := env.doJSON(t, http.MethodPut, "/api/characters/"+env.characterID+"/location",
		env.playerToken, map[string]any{"location_id": locationID})
	if assignRec.Code != http.StatusOK {
		t.Fatalf("assign: expected 200, got %d: %s", assignRec.Code, assignRec.Body.String())
	}

	var character struct {
		LocationID string `json:"location_id"`
	}
	if err := json.Unmarshal(assignRec.Body.Bytes(), &character); err != nil {
		t.Fatalf("decoding character: %v", err)
	}
	if character.LocationID != locationID {
		t.Fatalf("location_id = %q, want %q", character.LocationID, locationID)
	}
}
