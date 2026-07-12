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
		if err := users.Create(ctx, u); err != nil {
			t.Fatalf("Create user: %v", err)
		}
	}

	character := &models.Character{Name: "Aria", UserID: player.ID}
	if err := characters.Create(ctx, character); err != nil {
		t.Fatalf("Create character: %v", err)
	}

	group := &models.Group{Name: "Thieves Guild", Type: models.GroupTypeOrganization}
	if err := groups.Create(ctx, group); err != nil {
		t.Fatalf("Create group: %v", err)
	}
	entry := &models.ActivityEntry{
		Action: models.ActivityActionJoined, EntityType: "group", EntityID: group.ID,
		EntityName: group.Name, Actor: character.Name, CharacterID: character.ID,
	}
	if err := groups.AddMember(ctx, group, character, entry); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	router := transport.NewRouter(
		transport.WithHandle(
			"POST /api/characters/{id}/inventory",
			requireAuth(transport.AddInventoryItemHandler(inventorySvc, transport.CharacterOwner)),
		),
		transport.WithHandle(
			"GET /api/groups/{id}/inventory",
			requireAuth(transport.ListInventoryHandler(inventorySvc, transport.GroupOwner)),
		),
		transport.WithHandle("POST /api/inventory/move", requireAuth(transport.MoveInventoryItemHandler(inventorySvc))),
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

func TestInventoryMove_CharacterToGroup(t *testing.T) {
	env := newMoveTestEnv(t)

	addRec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/inventory",
		env.playerToken, map[string]any{"name": "Torch", "quantity": 3})
	if addRec.Code != http.StatusCreated {
		t.Fatalf("add: expected 201, got %d: %s", addRec.Code, addRec.Body.String())
	}

	var item struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(addRec.Body.Bytes(), &item); err != nil {
		t.Fatalf("decoding item: %v", err)
	}

	moveRec := env.doJSON(t, http.MethodPost, "/api/inventory/move", env.playerToken,
		map[string]any{"item_id": item.ID, "to_group_id": env.groupID, "quantity": 2})
	if moveRec.Code != http.StatusOK {
		t.Fatalf("move: expected 200, got %d: %s", moveRec.Code, moveRec.Body.String())
	}

	listRec := env.doJSON(t, http.MethodGet, "/api/groups/"+env.groupID+"/inventory", env.playerToken, nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var items []struct {
		Name     string `json:"name"`
		Quantity int    `json:"quantity"`
		GroupID  string `json:"group_id"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &items); err != nil {
		t.Fatalf("decoding items: %v", err)
	}
	if len(items) != 1 || items[0].Quantity != 2 || items[0].GroupID != env.groupID {
		t.Fatalf("group inventory = %+v, want one Torch x2", items)
	}
}

func TestInventoryMove_ForeignItemIs404(t *testing.T) {
	env := newMoveTestEnv(t)

	addRec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/inventory",
		env.playerToken, map[string]any{"name": "Torch", "quantity": 3})
	if addRec.Code != http.StatusCreated {
		t.Fatalf("add: expected 201, got %d: %s", addRec.Code, addRec.Body.String())
	}

	var item struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(addRec.Body.Bytes(), &item); err != nil {
		t.Fatalf("decoding item: %v", err)
	}

	// Another player must not be able to move — or detect — the item.
	rec := env.doJSON(t, http.MethodPost, "/api/inventory/move", env.otherToken,
		map[string]any{"item_id": item.ID, "to_group_id": env.groupID, "quantity": 1})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for foreign item, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGroupInventory_NonMemberGets404(t *testing.T) {
	env := newMoveTestEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/groups/"+env.groupID+"/inventory", env.otherToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-member, got %d: %s", rec.Code, rec.Body.String())
	}
}
