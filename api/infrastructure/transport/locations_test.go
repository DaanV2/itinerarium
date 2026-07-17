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

type locationHTTPTestEnv struct {
	router      *transport.Router
	gmToken     string
	playerToken string
	characterID string
}

func newLocationHTTPTestEnv(t *testing.T) locationHTTPTestEnv {
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
		require.NoError(t, users.Create(ctx, u), "Create user")
	}

	character := &models.Character{Name: "Aria", UserID: player.ID}
	require.NoError(t, characters.Create(ctx, character), "Create character")

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

func (e locationHTTPTestEnv) createLocation(t *testing.T, name string) string {
	t.Helper()

	rec := e.doJSON(t, http.MethodPost, "/api/locations", e.gmToken,
		map[string]any{"name": name, "plane": "Material"})
	require.Equal(t, http.StatusCreated, rec.Code, "create location body: %s", rec.Body.String())

	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created), "decoding location")

	return created.ID
}

func TestLocations_CreateIsGMOnly(t *testing.T) {
	env := newLocationHTTPTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/locations", env.playerToken,
		map[string]any{"name": "The Tavern"})
	require.Equal(t, http.StatusForbidden, rec.Code, "player create body: %s", rec.Body.String())
}

func TestLocations_HiddenWithoutGrant(t *testing.T) {
	env := newLocationHTTPTestEnv(t)
	locationID := env.createLocation(t, "Hidden Vault")

	// Absent from the list…
	listRec := env.doJSON(t, http.MethodGet, "/api/locations", env.playerToken, nil)
	require.Equal(t, http.StatusOK, listRec.Code, "list body: %s", listRec.Body.String())
	require.Contains(t, []string{"[]", "null"}, listRec.Body.String(),
		"list leaked locations to player without grants")

	// …direct read is 404, not 403.
	getRec := env.doJSON(t, http.MethodGet, "/api/locations/"+locationID, env.playerToken, nil)
	require.Equal(t, http.StatusNotFound, getRec.Code, "get body: %s", getRec.Body.String())

	// …and the access list is GM-only.
	accessRec := env.doJSON(t, http.MethodGet, "/api/locations/"+locationID+"/access", env.playerToken, nil)
	require.Equal(t, http.StatusForbidden, accessRec.Code, "access list body: %s", accessRec.Body.String())
}

func TestLocations_GrantThenVisibleAndAssignable(t *testing.T) {
	env := newLocationHTTPTestEnv(t)
	locationID := env.createLocation(t, "The Tavern")

	grantRec := env.doJSON(t, http.MethodPost, "/api/locations/"+locationID+"/access", env.gmToken,
		map[string]any{"character_id": env.characterID})
	require.Equal(t, http.StatusCreated, grantRec.Code, "grant body: %s", grantRec.Body.String())

	getRec := env.doJSON(t, http.MethodGet, "/api/locations/"+locationID, env.playerToken, nil)
	require.Equal(t, http.StatusOK, getRec.Code, "get with grant body: %s", getRec.Body.String())

	assignRec := env.doJSON(t, http.MethodPut, "/api/characters/"+env.characterID+"/location",
		env.playerToken, map[string]any{"location_id": locationID})
	require.Equal(t, http.StatusOK, assignRec.Code, "assign body: %s", assignRec.Body.String())

	var character struct {
		LocationID string `json:"location_id"`
	}
	require.NoError(t, json.Unmarshal(assignRec.Body.Bytes(), &character), "decoding character")
	require.Equal(t, locationID, character.LocationID)
}
