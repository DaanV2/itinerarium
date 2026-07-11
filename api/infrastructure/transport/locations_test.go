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
	authSvc := application.NewAuthService(tokens, users)
	locationSvc := application.NewLocationService(repositories.NewLocations(db))
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
		transport.WithHandle("DELETE /api/locations/{id}", requireAuth(transport.DeleteLocationHandler(locationSvc))),
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

func (e locationsTestEnv) createLocation(t *testing.T, payload any) string {
	t.Helper()

	rec := e.doJSON(t, http.MethodPost, "/api/locations", e.gmToken, payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create location: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created struct{ ID string }
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding body: %v", err)
	}

	return created.ID
}

func TestCreateLocation_RequiresAuth(t *testing.T) {
	env := newLocationsTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/locations", "", map[string]string{"name": "Neverwinter"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no token, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateLocation_GMCreatesPlane(t *testing.T) {
	env := newLocationsTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/locations", env.gmToken,
		map[string]string{"name": "The Material Plane", "description": "The mortal world."})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		ParentID    *string `json:"parent_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if created.Name != "The Material Plane" || created.Description != "The mortal world." {
		t.Fatalf("location = %+v, want name/description set", created)
	}
	if created.ParentID != nil {
		t.Fatalf("ParentID = %v, want nil", created.ParentID)
	}
}

func TestCreateLocation_PlayerForbidden(t *testing.T) {
	env := newLocationsTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/locations", env.playerToken,
		map[string]string{"name": "Neverwinter"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateLocation_UnknownParentIsBadRequest(t *testing.T) {
	env := newLocationsTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/locations", env.gmToken,
		map[string]string{"name": "Neverwinter", "parent_id": "does-not-exist"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListLocations_PlayerSeesAll(t *testing.T) {
	env := newLocationsTestEnv(t)

	env.createLocation(t, map[string]string{"name": "The Material Plane"})
	env.createLocation(t, map[string]string{"name": "The Feywild"})

	rec := env.doJSON(t, http.MethodGet, "/api/locations", env.playerToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var list []struct{ Name string }
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(list))
	}
}

func TestUpdateLocation_PlayerForbidden(t *testing.T) {
	env := newLocationsTestEnv(t)

	id := env.createLocation(t, map[string]string{"name": "Neverwinter"})

	rec := env.doJSON(t, http.MethodPatch, "/api/locations/"+id, env.playerToken,
		map[string]string{"name": "Hijacked"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateLocation_GMEdits(t *testing.T) {
	env := newLocationsTestEnv(t)

	id := env.createLocation(t, map[string]string{"name": "Neverwinter", "description": "A city."})

	rec := env.doJSON(t, http.MethodPatch, "/api/locations/"+id, env.gmToken,
		map[string]string{"description": "A ruined city."})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if updated.Description != "A ruined city." {
		t.Fatalf("Description = %q, want %q", updated.Description, "A ruined city.")
	}
}

func TestDeleteLocation_PlayerForbidden(t *testing.T) {
	env := newLocationsTestEnv(t)

	id := env.createLocation(t, map[string]string{"name": "Neverwinter"})

	rec := env.doJSON(t, http.MethodDelete, "/api/locations/"+id, env.playerToken, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteLocation_GMRemovesLeaf(t *testing.T) {
	env := newLocationsTestEnv(t)

	id := env.createLocation(t, map[string]string{"name": "Neverwinter"})

	rec := env.doJSON(t, http.MethodDelete, "/api/locations/"+id, env.gmToken, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	getRec := env.doJSON(t, http.MethodGet, "/api/locations/"+id, env.gmToken, nil)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d: %s", getRec.Code, getRec.Body.String())
	}
}

func TestDeleteLocation_WithChildrenConflicts(t *testing.T) {
	env := newLocationsTestEnv(t)

	planeID := env.createLocation(t, map[string]string{"name": "The Material Plane"})
	env.createLocation(t, map[string]any{"name": "Neverwinter", "parent_id": planeID})

	rec := env.doJSON(t, http.MethodDelete, "/api/locations/"+planeID, env.gmToken, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}
