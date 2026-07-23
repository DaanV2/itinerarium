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
	"github.com/stretchr/testify/require"
)

type locationHTTPTestEnv struct {
	router           *transport.Router
	gmToken          string
	playerToken      string
	otherPlayerToken string
	characterID      string
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
	otherPlayer := &models.User{Email: "other@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	for _, u := range []*models.User{gm, player, otherPlayer} {
		require.NoError(t, users.Create(ctx, u), "Create user")
	}

	character := &models.Character{Name: "Aria", UserID: player.ID}
	require.NoError(t, characters.Create(ctx, character), "Create character")

	router := transport.NewRouter(
		transport.WithHandle("GET /api/locations", requireAuth(handlers.ListLocationsHandler(locationSvc))),
		transport.WithHandle("POST /api/locations", requireAuth(handlers.CreateLocationHandler(locationSvc))),
		transport.WithHandle("GET /api/locations/{id}", requireAuth(handlers.GetLocationHandler(locationSvc))),
		transport.WithHandle("PATCH /api/locations/{id}", requireAuth(handlers.UpdateLocationHandler(locationSvc))),
		transport.WithHandle(
			"POST /api/locations/{id}/access", requireAuth(handlers.GrantLocationAccessHandler(locationSvc)),
		),
		transport.WithHandle(
			"GET /api/locations/{id}/access", requireAuth(handlers.ListLocationAccessHandler(locationSvc)),
		),
		transport.WithHandle(
			"PUT /api/characters/{id}/location", requireAuth(handlers.SetCharacterLocationHandler(locationSvc)),
		),
	)

	return locationHTTPTestEnv{
		router:           router,
		gmToken:          issueToken(t, tokens, gm.ID),
		playerToken:      issueToken(t, tokens, player.ID),
		otherPlayerToken: issueToken(t, tokens, otherPlayer.ID),
		characterID:      character.ID,
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

func TestLocations_PlayerWithAccessCanEditDescription(t *testing.T) {
	env := newLocationHTTPTestEnv(t)
	locationID := env.createLocation(t, "The Tavern")

	grantRec := env.doJSON(t, http.MethodPost, "/api/locations/"+locationID+"/access", env.gmToken,
		map[string]any{"character_id": env.characterID})
	require.Equal(t, http.StatusCreated, grantRec.Code, "grant body: %s", grantRec.Body.String())

	// A player without access is not-found, never forbidden…
	deniedRec := env.doJSON(t, http.MethodPatch, "/api/locations/"+locationID, env.otherPlayerToken,
		map[string]any{"name": "Should not apply"})
	require.Equal(t, http.StatusNotFound, deniedRec.Code, "denied edit body: %s", deniedRec.Body.String())

	// …but the granted player can edit both the name and the description.
	editRec := env.doJSON(t, http.MethodPatch, "/api/locations/"+locationID, env.playerToken,
		map[string]any{
			"name":     "The Rusty Tavern",
			"sections": []map[string]any{{"content": "Smells of stale ale."}},
		})
	require.Equal(t, http.StatusOK, editRec.Code, "edit body: %s", editRec.Body.String())

	var updated struct {
		Name     string `json:"name"`
		Sections []struct {
			Content string `json:"content"`
			GMOnly  bool   `json:"gm_only"`
		} `json:"sections"`
	}
	require.NoError(t, json.Unmarshal(editRec.Body.Bytes(), &updated), "decoding location")
	require.Equal(t, "The Rusty Tavern", updated.Name)
	require.Len(t, updated.Sections, 1)
	require.Equal(t, "Smells of stale ale.", updated.Sections[0].Content)
}

func TestLocations_GMOnlySectionStrippedForPlayers(t *testing.T) {
	env := newLocationHTTPTestEnv(t)
	locationID := env.createLocation(t, "The Tavern")

	grantRec := env.doJSON(t, http.MethodPost, "/api/locations/"+locationID+"/access", env.gmToken,
		map[string]any{"character_id": env.characterID})
	require.Equal(t, http.StatusCreated, grantRec.Code, "grant body: %s", grantRec.Body.String())

	editRec := env.doJSON(t, http.MethodPatch, "/api/locations/"+locationID, env.gmToken,
		map[string]any{"sections": []map[string]any{
			{"content": "A cosy tavern by the docks."},
			{"content": "The barkeep is a Guild informant.", "gm_only": true},
		}})
	require.Equal(t, http.StatusOK, editRec.Code, "edit body: %s", editRec.Body.String())

	getRec := env.doJSON(t, http.MethodGet, "/api/locations/"+locationID, env.playerToken, nil)
	require.Equal(t, http.StatusOK, getRec.Code, "get body: %s", getRec.Body.String())

	var got struct {
		Sections []struct {
			Content string `json:"content"`
			GMOnly  bool   `json:"gm_only"`
		} `json:"sections"`
	}
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &got), "decoding location")
	require.Len(t, got.Sections, 1, "GM-only section leaked to a player")
	require.Equal(t, "A cosy tavern by the docks.", got.Sections[0].Content)
}
