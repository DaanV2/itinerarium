package application_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// otherPlayer is a second, unrelated player used to prove existence hiding.
var otherPlayer = fakeRequester{id: "player-2", gm: false}

func newTestInventoryEnv(t *testing.T) (*application.InventoryService, *application.CatalogService, *application.CharacterService) {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err)
	err = db.Migrate()
	require.NoError(t, err)

	characters := repositories.NewCharacters(db)
	groups := repositories.NewGroups(db)
	currencies := repositories.NewCurrencies(db)
	itemDefs := repositories.NewItemDefinitions(db)
	charSvc := application.NewCharacterService(characters, repositories.NewUsers(db), repositories.NewKnowledgeRepositories(db))
	locationSvc := application.NewLocationService(
		repositories.NewLocations(db), repositories.NewLocationAccesses(db), groups, characters, charSvc,
	)
	catalogSvc := application.NewCatalogService(currencies, itemDefs)
	invSvc := application.NewInventoryService(
		charSvc,
		locationSvc,
		groups,
		characters,
		repositories.NewInventoryItems(db),
		repositories.NewMoneyBalances(db),
		currencies,
		itemDefs,
	)

	return invSvc, catalogSvc, charSvc
}

// ownedCharacter creates a character belonging to playerRequester.
func ownedCharacter(t *testing.T, charSvc *application.CharacterService, name string) *models.Character {
	t.Helper()

	c, err := charSvc.Create(t.Context(), playerRequester, "", name)
	require.NoError(t, err)

	return c
}

func TestInventoryService_AddItem_FreeTextAllowed(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	item, err := inv.AddItem(ctx, playerRequester, models.CharacterOwner(c.ID), "Mysterious Trinket", nil, 1, "found in a ditch")
	require.NoError(t, err)
	assert.Nil(t, item.ItemDefinitionID, "want nil for free-text item")
}

func TestInventoryService_AddItem_WithCatalogDefinition(t *testing.T) {
	inv, catalog, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	def, err := catalog.CreateItemDefinition(ctx, gmRequester, "Torch", "", "gear")
	require.NoError(t, err)

	item, err := inv.AddItem(ctx, playerRequester, models.CharacterOwner(c.ID), "Torch", &def.ID, 3, "")
	require.NoError(t, err)
	if assert.NotNil(t, item.ItemDefinitionID) {
		assert.Equal(t, def.ID, *item.ItemDefinitionID)
	}
}

func TestInventoryService_AddItem_UnknownDefinitionRejected(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	missing := "does-not-exist"

	_, err := inv.AddItem(ctx, playerRequester, models.CharacterOwner(c.ID), "Torch", &missing, 1, "")
	require.ErrorIs(t, err, application.ErrUnknownItemDefinition)
}

func TestInventoryService_AddItem_RejectsEmptyNameAndBadQuantity(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	_, err := inv.AddItem(ctx, playerRequester, models.CharacterOwner(c.ID), "", nil, 1, "")
	require.ErrorIs(t, err, application.ErrInvalidName)

	_, err = inv.AddItem(ctx, playerRequester, models.CharacterOwner(c.ID), "Torch", nil, 0, "")
	require.ErrorIs(t, err, application.ErrInvalidQuantity)
}

func TestInventoryService_AddItem_OtherPlayersCharacterHidden(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	_, err := inv.AddItem(ctx, otherPlayer, models.CharacterOwner(c.ID), "Torch", nil, 1, "")
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestInventoryService_ListInventory_OtherPlayerGetsNotFound(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	_, err := inv.AddItem(ctx, playerRequester, models.CharacterOwner(c.ID), "Torch", nil, 1, "")
	require.NoError(t, err)

	_, err = inv.ListInventory(ctx, otherPlayer, models.CharacterOwner(c.ID))
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestInventoryService_ListInventory_GMSeesOwnedByOthers(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	_, err := inv.AddItem(ctx, playerRequester, models.CharacterOwner(c.ID), "Torch", nil, 1, "")
	require.NoError(t, err)

	items, err := inv.ListInventory(ctx, gmRequester, models.CharacterOwner(c.ID))
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestInventoryService_UpdateItem_ChangesQuantity(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	item, err := inv.AddItem(ctx, playerRequester, models.CharacterOwner(c.ID), "Torch", nil, 1, "")
	require.NoError(t, err)

	qty := 5

	updated, err := inv.UpdateItem(ctx, playerRequester, models.CharacterOwner(c.ID), item.ID, nil, &qty, nil)
	require.NoError(t, err)
	assert.Equal(t, 5, updated.Quantity)
}

func TestInventoryService_UpdateItem_RejectsZeroQuantity(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	item, err := inv.AddItem(ctx, playerRequester, models.CharacterOwner(c.ID), "Torch", nil, 1, "")
	require.NoError(t, err)

	qty := 0

	_, err = inv.UpdateItem(ctx, playerRequester, models.CharacterOwner(c.ID), item.ID, nil, &qty, nil)
	require.ErrorIs(t, err, application.ErrInvalidQuantity)
}

func TestInventoryService_UpdateItem_ItemFromAnotherCharacterHidden(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()

	// Both characters belong to the same player, so access passes — but the
	// item still must not be reachable through the wrong character's path.
	charA := ownedCharacter(t, charSvc, "Aria")
	charB := ownedCharacter(t, charSvc, "Beren")

	item, err := inv.AddItem(ctx, playerRequester, models.CharacterOwner(charA.ID), "Torch", nil, 1, "")
	require.NoError(t, err)

	name := "Hijacked"

	_, err = inv.UpdateItem(ctx, playerRequester, models.CharacterOwner(charB.ID), item.ID, &name, nil, nil)
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestInventoryService_RemoveItem_OtherPlayerGetsNotFound(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	item, err := inv.AddItem(ctx, playerRequester, models.CharacterOwner(c.ID), "Torch", nil, 1, "")
	require.NoError(t, err)

	err = inv.RemoveItem(ctx, otherPlayer, models.CharacterOwner(c.ID), item.ID)
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestInventoryService_RemoveItem_Removes(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	item, err := inv.AddItem(ctx, playerRequester, models.CharacterOwner(c.ID), "Torch", nil, 1, "")
	require.NoError(t, err)
	err = inv.RemoveItem(ctx, playerRequester, models.CharacterOwner(c.ID), item.ID)
	require.NoError(t, err)

	items, err := inv.ListInventory(ctx, playerRequester, models.CharacterOwner(c.ID))
	require.NoError(t, err)
	assert.Empty(t, items, "Inventory should be empty after item removal")
}

func TestInventoryService_SetMoney_UpsertsBalance(t *testing.T) {
	inv, catalog, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	gp, err := catalog.CreateCurrency(ctx, gmRequester, "gp", "Gold", 100)
	require.NoError(t, err)

	_, err = inv.SetMoney(ctx, playerRequester, models.CharacterOwner(c.ID), gp.ID, 50)
	require.NoError(t, err)

	balance, err := inv.SetMoney(ctx, playerRequester, models.CharacterOwner(c.ID), gp.ID, 75)
	require.NoError(t, err)
	assert.Equal(t, int64(75), balance.Amount)

	balances, err := inv.ListMoney(ctx, playerRequester, models.CharacterOwner(c.ID))
	require.NoError(t, err)
	assert.Len(t, balances, 1, "ListMoney should upsert, not duplicate")
}

func TestInventoryService_SetMoney_UnknownCurrencyRejected(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	_, err := inv.SetMoney(ctx, playerRequester, models.CharacterOwner(c.ID), "does-not-exist", 50)
	require.ErrorIs(t, err, application.ErrUnknownCurrency)
}

func TestInventoryService_SetMoney_RejectsNegativeAmount(t *testing.T) {
	inv, catalog, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	gp, err := catalog.CreateCurrency(ctx, gmRequester, "gp", "Gold", 100)
	require.NoError(t, err)

	_, err = inv.SetMoney(ctx, playerRequester, models.CharacterOwner(c.ID), gp.ID, -1)
	require.ErrorIs(t, err, application.ErrInvalidAmount)
}

func TestInventoryService_ListMoney_OtherPlayerGetsNotFound(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	_, err := inv.ListMoney(ctx, otherPlayer, models.CharacterOwner(c.ID))
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestInventoryService_SetMoney_OtherPlayerGetsNotFound(t *testing.T) {
	inv, catalog, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	gp, err := catalog.CreateCurrency(ctx, gmRequester, "gp", "Gold", 100)
	require.NoError(t, err)

	_, err = inv.SetMoney(ctx, otherPlayer, models.CharacterOwner(c.ID), gp.ID, 50)
	require.ErrorIs(t, err, application.ErrNotFound)
}
