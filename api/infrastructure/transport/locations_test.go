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

type locationsTestEnv struct {
	router      *transport.Router
	gmToken     string
	playerToken string
}

func newLocationsTestEnv(t *testing.T) locationsTestEnv {
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
	locationSvc := application.NewLocationService(repositories.NewLocations(db))
	authSvc := application.NewAuthService(tokens, users)
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

	gmToken, err := tokens.Issue(gm.ID)
	if err != nil {
		t.Fatalf("Issue(gm): %v", err)
	}

	playerToken, err := tokens.Issue(player.ID)
	if err != nil {
		t.Fatalf("Issue(player): %v", err)
	}

	router := transport.NewRouter(
		transport.WithHandle("GET /api/locations", requireAuth(transport.ListLocationsHandler(locationSvc))),
		transport.WithHandle("POST /api/locations", requireAuth(transport.CreateLocationHandler(locationSvc))),
		transport.WithHandle("GET /api/locations/{id}", requireAuth(transport.GetLocationHandler(locationSvc))),
		transport.WithHandle("PATCH /api/locations/{id}", requireAuth(transport.UpdateLocationHandler(locationSvc))),
	)

	return locationsTestEnv{router: router, gmToken: gmToken, playerToken: playerToken}
}

func (e locationsTestEnv) doJSON(t *testing.T, method, path, token string, payload any) *httptest.ResponseRecorder {
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

func TestCreateLocation_RequiresAuth(t *testing.T) {
	env := newLocationsTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/locations", "", map[string]string{"name": "Waterdeep"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateLocation_PlayerForbidden(t *testing.T) {
	env := newLocationsTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/locations", env.playerToken, map[string]string{"name": "Waterdeep"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateLocation_GM(t *testing.T) {
	env := newLocationsTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/locations", env.gmToken,
		map[string]string{"name": "Waterdeep", "plane": "Material"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Plane string `json:"plane"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if created.Name != "Waterdeep" || created.Plane != "Material" {
		t.Fatalf("created = %+v", created)
	}
}

func TestListLocations_PlayerCanRead(t *testing.T) {
	env := newLocationsTestEnv(t)

	if rec := env.doJSON(t, http.MethodPost, "/api/locations", env.gmToken,
		map[string]string{"name": "Waterdeep"}); rec.Code != http.StatusCreated {
		t.Fatalf("seed: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	rec := env.doJSON(t, http.MethodGet, "/api/locations", env.playerToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var list []struct{ Name string }
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 location, got %d", len(list))
	}
}

func TestGetLocation_Unknown(t *testing.T) {
	env := newLocationsTestEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/locations/does-not-exist", env.playerToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
