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

type inventoryTestEnv struct {
	router      *transport.Router
	gmToken     string
	playerToken string
	otherToken  string
	characterID string
}

func newInventoryTestEnv(t *testing.T) inventoryTestEnv {
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
	currencies := repositories.NewCurrencies(db)
	itemDefs := repositories.NewItemDefinitions(db)
	authSvc := application.NewAuthService(tokens, users)
	characterSvc := application.NewCharacterService(characters, users, repositories.NewKnowledgeRepositories(db))
	groups := repositories.NewGroups(db)
	locationSvc := application.NewLocationService(
		repositories.NewLocations(db), repositories.NewLocationAccesses(db), groups, characters, characterSvc,
	)
	catalogSvc := application.NewCatalogService(currencies, itemDefs)
	inventorySvc := application.NewInventoryService(
		characterSvc, locationSvc, groups, characters,
		repositories.NewInventoryItems(db), repositories.NewMoneyBalances(db), currencies, itemDefs,
	)
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
		transport.WithHandle("GET /api/currencies", requireAuth(transport.ListCurrenciesHandler(catalogSvc))),
		transport.WithHandle("POST /api/currencies", requireAuth(transport.CreateCurrencyHandler(catalogSvc))),
		transport.WithHandle("POST /api/currencies/convert", requireAuth(transport.ConvertCurrencyHandler(catalogSvc))),
		transport.WithHandle("POST /api/currencies/simplify", requireAuth(transport.SimplifyCurrencyHandler(catalogSvc))),
		transport.WithHandle("GET /api/items", requireAuth(transport.ListItemDefinitionsHandler(catalogSvc))),
		transport.WithHandle("POST /api/items", requireAuth(transport.CreateItemDefinitionHandler(catalogSvc))),
		transport.WithHandle(
			"GET /api/characters/{id}/inventory",
			requireAuth(transport.ListInventoryHandler(inventorySvc, transport.CharacterOwner)),
		),
		transport.WithHandle(
			"POST /api/characters/{id}/inventory",
			requireAuth(transport.AddInventoryItemHandler(inventorySvc, transport.CharacterOwner)),
		),
		transport.WithHandle(
			"DELETE /api/characters/{id}/inventory/{itemId}",
			requireAuth(transport.RemoveInventoryItemHandler(inventorySvc, transport.CharacterOwner)),
		),
		transport.WithHandle(
			"GET /api/characters/{id}/money",
			requireAuth(transport.ListMoneyHandler(inventorySvc, transport.CharacterOwner)),
		),
		transport.WithHandle(
			"PUT /api/characters/{id}/money/{currencyId}",
			requireAuth(transport.SetMoneyHandler(inventorySvc, transport.CharacterOwner)),
		),
	)

	return inventoryTestEnv{
		router:      router,
		gmToken:     issueToken(t, tokens, gm.ID),
		playerToken: issueToken(t, tokens, player.ID),
		otherToken:  issueToken(t, tokens, other.ID),
		characterID: character.ID,
	}
}

func issueToken(t *testing.T, tokens *authentication.TokenService, userID string) string {
	t.Helper()

	token, err := tokens.Issue(userID)
	if err != nil {
		t.Fatalf("Issue(%s): %v", userID, err)
	}

	return token
}

func (e inventoryTestEnv) doJSON(t *testing.T, method, path, token string, payload any) *httptest.ResponseRecorder {
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

func TestInventory_RequiresAuth(t *testing.T) {
	env := newInventoryTestEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/characters/"+env.characterID+"/inventory", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no token, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestInventory_AddAndListItem(t *testing.T) {
	env := newInventoryTestEnv(t)

	addRec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/inventory", env.playerToken,
		map[string]any{"name": "Torch", "quantity": 3})
	if addRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", addRec.Code, addRec.Body.String())
	}

	listRec := env.doJSON(t, http.MethodGet, "/api/characters/"+env.characterID+"/inventory", env.playerToken, nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var items []struct {
		Name     string `json:"name"`
		Quantity int    `json:"quantity"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &items); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if len(items) != 1 || items[0].Name != "Torch" || items[0].Quantity != 3 {
		t.Fatalf("items = %+v, want one Torch x3", items)
	}
}

func TestInventory_OtherPlayerGets404(t *testing.T) {
	env := newInventoryTestEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/characters/"+env.characterID+"/inventory", env.otherToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for another player, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestInventory_AddItemForOtherPlayerGets404(t *testing.T) {
	env := newInventoryTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/inventory", env.otherToken,
		map[string]any{"name": "Torch", "quantity": 1})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for another player, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMoney_SetAndList(t *testing.T) {
	env := newInventoryTestEnv(t)

	currencyRec := env.doJSON(t, http.MethodPost, "/api/currencies", env.gmToken,
		map[string]any{"code": "gp", "name": "Gold", "ratio": 100})
	if currencyRec.Code != http.StatusCreated {
		t.Fatalf("CreateCurrency expected 201, got %d: %s", currencyRec.Code, currencyRec.Body.String())
	}

	var currency struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(currencyRec.Body.Bytes(), &currency); err != nil {
		t.Fatalf("decoding currency: %v", err)
	}

	setRec := env.doJSON(t, http.MethodPut,
		"/api/characters/"+env.characterID+"/money/"+currency.ID, env.playerToken,
		map[string]any{"amount": 42})
	if setRec.Code != http.StatusOK {
		t.Fatalf("SetMoney expected 200, got %d: %s", setRec.Code, setRec.Body.String())
	}

	listRec := env.doJSON(t, http.MethodGet, "/api/characters/"+env.characterID+"/money", env.playerToken, nil)

	var balances []struct {
		Amount int64 `json:"amount"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &balances); err != nil {
		t.Fatalf("decoding balances: %v", err)
	}
	if len(balances) != 1 || balances[0].Amount != 42 {
		t.Fatalf("balances = %+v, want one balance of 42", balances)
	}
}

func TestCurrency_PlayerCannotCreate(t *testing.T) {
	env := newInventoryTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/currencies", env.playerToken,
		map[string]any{"code": "gp", "name": "Gold", "ratio": 100})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for player creating currency, got %d: %s", rec.Code, rec.Body.String())
	}
}

func seedGoldSilverCopper(t *testing.T, env inventoryTestEnv) {
	t.Helper()

	for _, c := range []map[string]any{
		{"code": "cp", "name": "Copper", "ratio": 1},
		{"code": "sp", "name": "Silver", "ratio": 10},
		{"code": "gp", "name": "Gold", "ratio": 100},
	} {
		rec := env.doJSON(t, http.MethodPost, "/api/currencies", env.gmToken, c)
		if rec.Code != http.StatusCreated {
			t.Fatalf("seeding currency %v: %d %s", c, rec.Code, rec.Body.String())
		}
	}
}

func TestCurrency_Convert_PlayerCanUse(t *testing.T) {
	env := newInventoryTestEnv(t)
	seedGoldSilverCopper(t, env)

	rec := env.doJSON(t, http.MethodPost, "/api/currencies/convert", env.playerToken,
		map[string]any{"amounts": []map[string]any{{"currency": "gp", "amount": 5}}, "to": "sp"})
	if rec.Code != http.StatusOK {
		t.Fatalf("Convert expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Whole     int64 `json:"whole"`
		Remainder int64 `json:"remainder"`
		BaseValue int64 `json:"base_value"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if body.Whole != 50 || body.Remainder != 0 || body.BaseValue != 500 {
		t.Fatalf("Convert body = %+v, want whole 50 remainder 0 base 500", body)
	}
}

func TestCurrency_Convert_UnknownCurrencyIs404(t *testing.T) {
	env := newInventoryTestEnv(t)
	seedGoldSilverCopper(t, env)

	rec := env.doJSON(t, http.MethodPost, "/api/currencies/convert", env.playerToken,
		map[string]any{"amounts": []map[string]any{{"currency": "nope", "amount": 5}}, "to": "sp"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown currency, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCurrency_Simplify_BreaksIntoDenominations(t *testing.T) {
	env := newInventoryTestEnv(t)
	seedGoldSilverCopper(t, env)

	rec := env.doJSON(t, http.MethodPost, "/api/currencies/simplify", env.playerToken,
		map[string]any{"amounts": []map[string]any{{"currency": "cp", "amount": 1234}}})
	if rec.Code != http.StatusOK {
		t.Fatalf("Simplify expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var breakdown []struct {
		Currency struct {
			Code string `json:"code"`
		} `json:"currency"`
		Amount int64 `json:"amount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &breakdown); err != nil {
		t.Fatalf("decoding body: %v", err)
	}

	want := map[string]int64{"gp": 12, "sp": 3, "cp": 4}
	if len(breakdown) != len(want) {
		t.Fatalf("Simplify returned %d entries, want %d: %+v", len(breakdown), len(want), breakdown)
	}
	for _, entry := range breakdown {
		if entry.Amount != want[entry.Currency.Code] {
			t.Fatalf("Simplify %s = %d, want %d", entry.Currency.Code, entry.Amount, want[entry.Currency.Code])
		}
	}
}
