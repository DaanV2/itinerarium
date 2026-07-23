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

type moveTestEnv struct {
	router      *transport.Router
	playerToken string
	otherToken  string
	characterID string
	groupID     string
}

func newMoveTestEnv(t *testing.T) moveTestEnv {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err, "persistence.New")
	require.NoError(t, db.Migrate(), "Migrate")

	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(t.TempDir()))
	require.NoError(t, err, "NewKeyStore")

	tokens := authentication.NewTokenService(keys, repositories.NewRevokedTokens(db))
	users := repositories.NewUsers(db)
	characters := repositories.NewCharacters(db)
	groups := repositories.NewGroups(db)
	currencies := repositories.NewCurrencies(db)
	itemDefs := repositories.NewItemDefinitions(db)
	authSvc := application.NewAuthService(tokens, users)
	characterSvc := application.NewCharacterService(characters, users, repositories.NewKnowledgeRepositories(db))
	locationSvc := application.NewLocationService(
		repositories.NewLocations(db), repositories.NewLocationAccesses(db), groups, characters, characterSvc,
	)
	inventorySvc := application.NewInventoryService(
		characterSvc, locationSvc, groups, characters,
		repositories.NewInventoryItems(db), repositories.NewMoneyBalances(db), currencies, itemDefs,
	)
	requireAuth := transport.RequireAuth(authSvc)

	ctx := t.Context()
	player := &models.User{Email: "player@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	other := &models.User{Email: "other@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	for _, u := range []*models.User{player, other} {
		require.NoError(t, users.Create(ctx, u), "Create user")
	}

	character := &models.Character{Name: "Aria", UserID: player.ID}
	require.NoError(t, characters.Create(ctx, character), "Create character")

	group := &models.Group{Name: "Thieves Guild", Type: models.GroupTypeOrganization}
	require.NoError(t, groups.Create(ctx, group), "Create group")

	entry := &models.ActivityEntry{
		Action: models.ActivityActionJoined, EntityType: "group", EntityID: group.ID,
		EntityName: group.Name, Actor: character.Name, CharacterID: character.ID,
	}
	require.NoError(t, groups.AddMember(ctx, group, character, entry), "AddMember")

	router := transport.NewRouter(
		transport.WithHandle(
			"POST /api/characters/{id}/inventory",
			requireAuth(handlers.AddInventoryItemHandler(inventorySvc, handlers.CharacterOwner)),
		),
		transport.WithHandle(
			"GET /api/groups/{id}/inventory",
			requireAuth(handlers.ListInventoryHandler(inventorySvc, handlers.GroupOwner)),
		),
		transport.WithHandle("POST /api/inventory/move", requireAuth(handlers.MoveInventoryItemHandler(inventorySvc))),
	)

	return moveTestEnv{
		router:      router,
		playerToken: issueToken(t, tokens, player.ID),
		otherToken:  issueToken(t, tokens, other.ID),
		characterID: character.ID,
		groupID:     group.ID,
	}
}

func (e moveTestEnv) doJSON(t *testing.T, method, path, token string, payload any) *httptest.ResponseRecorder {
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

func TestInventoryMove_CharacterToGroup(t *testing.T) {
	env := newMoveTestEnv(t)

	addRec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/inventory",
		env.playerToken, map[string]any{"name": "Torch", "quantity": 3})
	require.Equal(t, http.StatusCreated, addRec.Code, "add body: %s", addRec.Body.String())

	var item struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(addRec.Body.Bytes(), &item), "decoding item")

	moveRec := env.doJSON(t, http.MethodPost, "/api/inventory/move", env.playerToken,
		map[string]any{"item_id": item.ID, "to_group_id": env.groupID, "quantity": 2})
	require.Equal(t, http.StatusOK, moveRec.Code, "move body: %s", moveRec.Body.String())

	listRec := env.doJSON(t, http.MethodGet, "/api/groups/"+env.groupID+"/inventory", env.playerToken, nil)
	require.Equal(t, http.StatusOK, listRec.Code, "list body: %s", listRec.Body.String())

	var items []struct {
		Name     string `json:"name"`
		Quantity int    `json:"quantity"`
		GroupID  string `json:"group_id"`
	}
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &items), "decoding items")
	require.Len(t, items, 1, "want one Torch x2")
	require.Equal(t, 2, items[0].Quantity)
	require.Equal(t, env.groupID, items[0].GroupID)
}

func TestInventoryMove_ForeignItemIs404(t *testing.T) {
	env := newMoveTestEnv(t)

	addRec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/inventory",
		env.playerToken, map[string]any{"name": "Torch", "quantity": 3})
	require.Equal(t, http.StatusCreated, addRec.Code, "add body: %s", addRec.Body.String())

	var item struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(addRec.Body.Bytes(), &item), "decoding item")

	// Another player must not be able to move — or detect — the item.
	rec := env.doJSON(t, http.MethodPost, "/api/inventory/move", env.otherToken,
		map[string]any{"item_id": item.ID, "to_group_id": env.groupID, "quantity": 1})
	require.Equal(t, http.StatusNotFound, rec.Code, "foreign item body: %s", rec.Body.String())
}

func TestGroupInventory_NonMemberGets404(t *testing.T) {
	env := newMoveTestEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/groups/"+env.groupID+"/inventory", env.otherToken, nil)
	require.Equal(t, http.StatusNotFound, rec.Code, "non-member body: %s", rec.Body.String())
}
