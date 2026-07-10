package application_test

import (
	"errors"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// otherPlayer is a second, unrelated player used to prove existence hiding.
var otherPlayer = fakeRequester{id: "player-2", gm: false}

func newTestInventoryEnv(t *testing.T) (*application.InventoryService, *application.CatalogService, *application.CharacterService) {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	if err != nil {
		t.Fatalf("persistence.New: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	characters := repositories.NewCharacters(db)
	currencies := repositories.NewCurrencies(db)
	itemDefs := repositories.NewItemDefinitions(db)
	charSvc := application.NewCharacterService(characters, repositories.NewUsers(db))
	catalogSvc := application.NewCatalogService(currencies, itemDefs)
	invSvc := application.NewInventoryService(
		charSvc,
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
	if err != nil {
		t.Fatalf("Create character: %v", err)
	}

	return c
}

func TestInventoryService_AddItem_FreeTextAllowed(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	item, err := inv.AddItem(ctx, playerRequester, c.ID, "Mysterious Trinket", nil, 1, "found in a ditch")
	if err != nil {
		t.Fatalf("AddItem(free text): %v", err)
	}
	if item.ItemDefinitionID != nil {
		t.Fatalf("ItemDefinitionID = %v, want nil for free-text item", *item.ItemDefinitionID)
	}
}

func TestInventoryService_AddItem_WithCatalogDefinition(t *testing.T) {
	inv, catalog, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	def, err := catalog.CreateItemDefinition(ctx, gmRequester, "Torch", "", "gear")
	if err != nil {
		t.Fatalf("CreateItemDefinition: %v", err)
	}

	item, err := inv.AddItem(ctx, playerRequester, c.ID, "Torch", &def.ID, 3, "")
	if err != nil {
		t.Fatalf("AddItem(with def): %v", err)
	}
	if item.ItemDefinitionID == nil || *item.ItemDefinitionID != def.ID {
		t.Fatalf("ItemDefinitionID = %v, want %q", item.ItemDefinitionID, def.ID)
	}
}

func TestInventoryService_AddItem_UnknownDefinitionRejected(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	missing := "does-not-exist"

	_, err := inv.AddItem(ctx, playerRequester, c.ID, "Torch", &missing, 1, "")
	if !errors.Is(err, application.ErrUnknownItemDefinition) {
		t.Fatalf("AddItem(unknown def) = %v, want ErrUnknownItemDefinition", err)
	}
}

func TestInventoryService_AddItem_RejectsEmptyNameAndBadQuantity(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	if _, err := inv.AddItem(ctx, playerRequester, c.ID, "", nil, 1, ""); !errors.Is(err, application.ErrInvalidName) {
		t.Fatalf("AddItem(empty name) = %v, want ErrInvalidName", err)
	}
	if _, err := inv.AddItem(ctx, playerRequester, c.ID, "Torch", nil, 0, ""); !errors.Is(err, application.ErrInvalidQuantity) {
		t.Fatalf("AddItem(quantity 0) = %v, want ErrInvalidQuantity", err)
	}
}

func TestInventoryService_AddItem_OtherPlayersCharacterHidden(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	_, err := inv.AddItem(ctx, otherPlayer, c.ID, "Torch", nil, 1, "")
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("AddItem(other player) = %v, want ErrNotFound", err)
	}
}

func TestInventoryService_ListInventory_OtherPlayerGetsNotFound(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	if _, err := inv.AddItem(ctx, playerRequester, c.ID, "Torch", nil, 1, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	_, err := inv.ListInventory(ctx, otherPlayer, c.ID)
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("ListInventory(other player) = %v, want ErrNotFound", err)
	}
}

func TestInventoryService_ListInventory_GMSeesOwnedByOthers(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	if _, err := inv.AddItem(ctx, playerRequester, c.ID, "Torch", nil, 1, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	items, err := inv.ListInventory(ctx, gmRequester, c.ID)
	if err != nil {
		t.Fatalf("ListInventory(GM): %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListInventory(GM) returned %d items, want 1", len(items))
	}
}

func TestInventoryService_UpdateItem_ChangesQuantity(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	item, err := inv.AddItem(ctx, playerRequester, c.ID, "Torch", nil, 1, "")
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	qty := 5

	updated, err := inv.UpdateItem(ctx, playerRequester, c.ID, item.ID, nil, &qty, nil)
	if err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}
	if updated.Quantity != 5 {
		t.Fatalf("Quantity = %d, want 5", updated.Quantity)
	}
}

func TestInventoryService_UpdateItem_RejectsZeroQuantity(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	item, err := inv.AddItem(ctx, playerRequester, c.ID, "Torch", nil, 1, "")
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	qty := 0

	_, err = inv.UpdateItem(ctx, playerRequester, c.ID, item.ID, nil, &qty, nil)
	if !errors.Is(err, application.ErrInvalidQuantity) {
		t.Fatalf("UpdateItem(qty 0) = %v, want ErrInvalidQuantity", err)
	}
}

func TestInventoryService_UpdateItem_ItemFromAnotherCharacterHidden(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()

	// Both characters belong to the same player, so access passes — but the
	// item still must not be reachable through the wrong character's path.
	charA := ownedCharacter(t, charSvc, "Aria")
	charB := ownedCharacter(t, charSvc, "Beren")

	item, err := inv.AddItem(ctx, playerRequester, charA.ID, "Torch", nil, 1, "")
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	name := "Hijacked"

	_, err = inv.UpdateItem(ctx, playerRequester, charB.ID, item.ID, &name, nil, nil)
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("UpdateItem(cross-character) = %v, want ErrNotFound", err)
	}
}

func TestInventoryService_RemoveItem_OtherPlayerGetsNotFound(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	item, err := inv.AddItem(ctx, playerRequester, c.ID, "Torch", nil, 1, "")
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	err = inv.RemoveItem(ctx, otherPlayer, c.ID, item.ID)
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("RemoveItem(other player) = %v, want ErrNotFound", err)
	}
}

func TestInventoryService_RemoveItem_Removes(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	item, err := inv.AddItem(ctx, playerRequester, c.ID, "Torch", nil, 1, "")
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if err := inv.RemoveItem(ctx, playerRequester, c.ID, item.ID); err != nil {
		t.Fatalf("RemoveItem: %v", err)
	}

	items, err := inv.ListInventory(ctx, playerRequester, c.ID)
	if err != nil {
		t.Fatalf("ListInventory: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("ListInventory returned %d items after remove, want 0", len(items))
	}
}

func TestInventoryService_SetMoney_UpsertsBalance(t *testing.T) {
	inv, catalog, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	gp, err := catalog.CreateCurrency(ctx, gmRequester, "gp", "Gold", 100)
	if err != nil {
		t.Fatalf("CreateCurrency: %v", err)
	}

	if _, err := inv.SetMoney(ctx, playerRequester, c.ID, gp.ID, 50); err != nil {
		t.Fatalf("SetMoney(first): %v", err)
	}

	balance, err := inv.SetMoney(ctx, playerRequester, c.ID, gp.ID, 75)
	if err != nil {
		t.Fatalf("SetMoney(update): %v", err)
	}
	if balance.Amount != 75 {
		t.Fatalf("Amount = %d, want 75", balance.Amount)
	}

	balances, err := inv.ListMoney(ctx, playerRequester, c.ID)
	if err != nil {
		t.Fatalf("ListMoney: %v", err)
	}
	if len(balances) != 1 {
		t.Fatalf("ListMoney returned %d balances, want 1 (upsert, not duplicate)", len(balances))
	}
}

func TestInventoryService_SetMoney_UnknownCurrencyRejected(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	_, err := inv.SetMoney(ctx, playerRequester, c.ID, "does-not-exist", 50)
	if !errors.Is(err, application.ErrUnknownCurrency) {
		t.Fatalf("SetMoney(unknown currency) = %v, want ErrUnknownCurrency", err)
	}
}

func TestInventoryService_SetMoney_RejectsNegativeAmount(t *testing.T) {
	inv, catalog, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	gp, err := catalog.CreateCurrency(ctx, gmRequester, "gp", "Gold", 100)
	if err != nil {
		t.Fatalf("CreateCurrency: %v", err)
	}

	_, err = inv.SetMoney(ctx, playerRequester, c.ID, gp.ID, -1)
	if !errors.Is(err, application.ErrInvalidAmount) {
		t.Fatalf("SetMoney(negative) = %v, want ErrInvalidAmount", err)
	}
}

func TestInventoryService_ListMoney_OtherPlayerGetsNotFound(t *testing.T) {
	inv, _, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	_, err := inv.ListMoney(ctx, otherPlayer, c.ID)
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("ListMoney(other player) = %v, want ErrNotFound", err)
	}
}

func TestInventoryService_SetMoney_OtherPlayerGetsNotFound(t *testing.T) {
	inv, catalog, charSvc := newTestInventoryEnv(t)
	ctx := t.Context()
	c := ownedCharacter(t, charSvc, "Aria")

	gp, err := catalog.CreateCurrency(ctx, gmRequester, "gp", "Gold", 100)
	if err != nil {
		t.Fatalf("CreateCurrency: %v", err)
	}

	_, err = inv.SetMoney(ctx, otherPlayer, c.ID, gp.ID, 50)
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("SetMoney(other player) = %v, want ErrNotFound", err)
	}
}
