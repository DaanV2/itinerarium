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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	err = db.Migrate()
	require.NoError(t, err)

	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(t.TempDir()))
	require.NoError(t, err)

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
		require.NoError(t, users.Create(ctx, u))
	}

	character := &models.Character{Name: "Aria", UserID: player.ID}
	err = characters.Create(ctx, character)
	require.NoError(t, err)

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
	require.NoError(t, err)

	return token
}

func (e inventoryTestEnv) doJSON(t *testing.T, method, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	if payload != nil {
		err := json.NewEncoder(&body).Encode(payload)
		require.NoError(t, err, "encoding request payload")
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
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestInventory_AddAndListItem(t *testing.T) {
	env := newInventoryTestEnv(t)

	addRec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/inventory", env.playerToken,
		map[string]any{"name": "Torch", "quantity": 3})
	require.Equal(t, http.StatusCreated, addRec.Code, addRec.Body.String())

	listRec := env.doJSON(t, http.MethodGet, "/api/characters/"+env.characterID+"/inventory", env.playerToken, nil)
	require.Equal(t, http.StatusOK, listRec.Code, listRec.Body.String())

	var items []struct {
		Name     string `json:"name"`
		Quantity int    `json:"quantity"`
	}
	err := json.Unmarshal(listRec.Body.Bytes(), &items)
	require.NoError(t, err)
	if assert.Len(t, items, 1, "items = %+v, want one Torch x3", items) {
		assert.Equal(t, "Torch", items[0].Name)
		assert.Equal(t, 3, items[0].Quantity)
	}
}

func TestInventory_OtherPlayerGets404(t *testing.T) {
	env := newInventoryTestEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/characters/"+env.characterID+"/inventory", env.otherToken, nil)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestInventory_AddItemForOtherPlayerGets404(t *testing.T) {
	env := newInventoryTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/inventory", env.otherToken,
		map[string]any{"name": "Torch", "quantity": 1})
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestMoney_SetAndList(t *testing.T) {
	env := newInventoryTestEnv(t)

	currencyRec := env.doJSON(t, http.MethodPost, "/api/currencies", env.gmToken,
		map[string]any{"code": "gp", "name": "Gold", "ratio": 100})
	require.Equal(t, http.StatusCreated, currencyRec.Code, currencyRec.Body.String())

	var currency struct {
		ID string `json:"id"`
	}
	err := json.Unmarshal(currencyRec.Body.Bytes(), &currency)
	require.NoError(t, err)

	setRec := env.doJSON(t, http.MethodPut,
		"/api/characters/"+env.characterID+"/money/"+currency.ID, env.playerToken,
		map[string]any{"amount": 42})
	require.Equal(t, http.StatusOK, setRec.Code, setRec.Body.String())

	listRec := env.doJSON(t, http.MethodGet, "/api/characters/"+env.characterID+"/money", env.playerToken, nil)

	var balances []struct {
		Amount int64 `json:"amount"`
	}
	err = json.Unmarshal(listRec.Body.Bytes(), &balances)
	require.NoError(t, err)
	if assert.Len(t, balances, 1, "balances = %+v, want one balance of 42", balances) {
		assert.Equal(t, int64(42), balances[0].Amount)
	}
}

func TestCurrency_PlayerCannotCreate(t *testing.T) {
	env := newInventoryTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/currencies", env.playerToken,
		map[string]any{"code": "gp", "name": "Gold", "ratio": 100})
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func seedGoldSilverCopper(t *testing.T, env inventoryTestEnv) {
	t.Helper()

	for _, c := range []map[string]any{
		{"code": "cp", "name": "Copper", "ratio": 1},
		{"code": "sp", "name": "Silver", "ratio": 10},
		{"code": "gp", "name": "Gold", "ratio": 100},
	} {
		rec := env.doJSON(t, http.MethodPost, "/api/currencies", env.gmToken, c)
		require.Equal(t, http.StatusCreated, rec.Code)
	}
}

func TestCurrency_Convert_PlayerCanUse(t *testing.T) {
	env := newInventoryTestEnv(t)
	seedGoldSilverCopper(t, env)

	rec := env.doJSON(t, http.MethodPost, "/api/currencies/convert", env.playerToken,
		map[string]any{"amounts": []map[string]any{{"currency": "gp", "amount": 5}}, "to": "sp"})
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Whole     int64 `json:"whole"`
		Remainder int64 `json:"remainder"`
		BaseValue int64 `json:"base_value"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)

	assert.EqualValues(t, 50, body.Whole, "Convert body whole = %d, want 50", body.Whole)
	assert.EqualValues(t, 0, body.Remainder, "Convert body remainder = %d, want 0", body.Remainder)
	assert.EqualValues(t, 500, body.BaseValue, "Convert body base_value = %d, want 500", body.BaseValue)
}

func TestCurrency_Convert_UnknownCurrencyIs404(t *testing.T) {
	env := newInventoryTestEnv(t)
	seedGoldSilverCopper(t, env)

	rec := env.doJSON(t, http.MethodPost, "/api/currencies/convert", env.playerToken,
		map[string]any{"amounts": []map[string]any{{"currency": "nope", "amount": 5}}, "to": "sp"})
	require.Equal(t, http.StatusNotFound, rec.Code)

}

func TestCurrency_Simplify_BreaksIntoDenominations(t *testing.T) {
	env := newInventoryTestEnv(t)
	seedGoldSilverCopper(t, env)

	org := map[string]any{"amounts": []map[string]any{{"currency": "cp", "amount": 1234}}}
	rec := env.doJSON(t, http.MethodPost, "/api/currencies/simplify", env.playerToken, org)
	require.Equal(t, http.StatusOK, rec.Code, "Simplify expected 200, got %d: %s", rec.Code, rec.Body.String())

	var breakdown []struct {
		Currency struct {
			Code string `json:"code"`
		} `json:"currency"`
		Amount int64 `json:"amount"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &breakdown)
	require.NoError(t, err)

	want := map[string]int64{"gp": 12, "sp": 3, "cp": 4}

	require.Len(t, breakdown, len(want), "Simplify returned %d entries, want %d: %+v", len(breakdown), len(want), breakdown)

	for _, entry := range breakdown {
		require.Contains(t, want, entry.Currency.Code, "Simplify returned unexpected currency %s", entry.Currency.Code)
	}
}
