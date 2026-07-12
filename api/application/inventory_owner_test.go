package application_test

import (
	"errors"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

type ownerInventoryTestEnv struct {
	inventory *application.InventoryService
	catalog   *application.CatalogService
	chars     *application.CharacterService
	groups    *application.GroupService
	locations *application.LocationService
}

func newOwnerInventoryTestEnv(t *testing.T) ownerInventoryTestEnv {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	if err != nil {
		t.Fatalf("persistence.New: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	characters := repositories.NewCharacters(db)
	groups := repositories.NewGroups(db)
	currencies := repositories.NewCurrencies(db)
	itemDefs := repositories.NewItemDefinitions(db)
	charSvc := application.NewCharacterService(characters, repositories.NewUsers(db))
	locationSvc := application.NewLocationService(
		repositories.NewLocations(db), repositories.NewLocationAccesses(db), groups, characters, charSvc,
	)

	return ownerInventoryTestEnv{
		inventory: application.NewInventoryService(
			charSvc,
			locationSvc,
			groups,
			characters,
			repositories.NewInventoryItems(db),
			repositories.NewMoneyBalances(db),
			currencies,
			itemDefs,
		),
		catalog:   application.NewCatalogService(currencies, itemDefs),
		chars:     charSvc,
		groups:    application.NewGroupService(groups, charSvc),
		locations: locationSvc,
	}
}

// memberGroup creates a group and joins the given character to it.
func (e ownerInventoryTestEnv) memberGroup(t *testing.T, characterID string) *models.Group {
	t.Helper()

	group, err := e.groups.Create(t.Context(), gmRequester, "Thieves Guild", models.GroupTypeOrganization, "")
	if err != nil {
		t.Fatalf("Create group: %v", err)
	}
	if err := e.groups.Join(t.Context(), gmRequester, group.ID, characterID); err != nil {
		t.Fatalf("Join: %v", err)
	}

	return group
}

// grantedLocation creates a location and grants the given character access.
func (e ownerInventoryTestEnv) grantedLocation(t *testing.T, characterID string) *models.Location {
	t.Helper()

	location, err := e.locations.Create(t.Context(), gmRequester, "The Vault", "", "Material")
	if err != nil {
		t.Fatalf("Create location: %v", err)
	}
	if _, err := e.locations.GrantAccess(t.Context(), gmRequester, location.ID, &characterID, nil); err != nil {
		t.Fatalf("GrantAccess: %v", err)
	}

	return location
}

func TestInventoryService_GroupInventory_MembersOnly(t *testing.T) {
	env := newOwnerInventoryTestEnv(t)
	ctx := t.Context()
	character := ownedCharacter(t, env.chars, "Aria")
	group := env.memberGroup(t, character.ID)
	owner := models.GroupOwner(group.ID)

	if _, err := env.inventory.AddItem(ctx, playerRequester, owner, "Shared Rations", nil, 10, ""); err != nil {
		t.Fatalf("AddItem as member: %v", err)
	}

	items, err := env.inventory.ListInventory(ctx, playerRequester, owner)
	if err != nil {
		t.Fatalf("ListInventory as member: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListInventory returned %d items, want 1", len(items))
	}

	// A player without a member character gets 404 — the shared inventory is
	// member-only content even though the group itself is public.
	if _, err := env.inventory.ListInventory(ctx, otherPlayer, owner); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("ListInventory as non-member = %v, want ErrNotFound", err)
	}
	if _, err := env.inventory.AddItem(ctx, otherPlayer, owner, "Loot", nil, 1, ""); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("AddItem as non-member = %v, want ErrNotFound", err)
	}

	// GMs always have access.
	if _, err := env.inventory.ListInventory(ctx, gmRequester, owner); err != nil {
		t.Fatalf("ListInventory as GM: %v", err)
	}
}

func TestInventoryService_GroupMoney_MembersOnly(t *testing.T) {
	env := newOwnerInventoryTestEnv(t)
	ctx := t.Context()
	character := ownedCharacter(t, env.chars, "Aria")
	group := env.memberGroup(t, character.ID)
	owner := models.GroupOwner(group.ID)

	gp, err := env.catalog.CreateCurrency(ctx, gmRequester, "gp", "Gold", 100)
	if err != nil {
		t.Fatalf("CreateCurrency: %v", err)
	}

	if _, err := env.inventory.SetMoney(ctx, playerRequester, owner, gp.ID, 250); err != nil {
		t.Fatalf("SetMoney as member: %v", err)
	}

	balances, err := env.inventory.ListMoney(ctx, playerRequester, owner)
	if err != nil {
		t.Fatalf("ListMoney as member: %v", err)
	}
	if len(balances) != 1 || balances[0].Amount != 250 {
		t.Fatalf("balances = %+v, want one of 250", balances)
	}

	if _, err := env.inventory.ListMoney(ctx, otherPlayer, owner); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("ListMoney as non-member = %v, want ErrNotFound", err)
	}
}

func TestInventoryService_LocationInventory_GrantGated(t *testing.T) {
	env := newOwnerInventoryTestEnv(t)
	ctx := t.Context()
	character := ownedCharacter(t, env.chars, "Aria")
	location := env.grantedLocation(t, character.ID)
	owner := models.LocationOwner(location.ID)

	if _, err := env.inventory.AddItem(ctx, playerRequester, owner, "Stored Gear", nil, 2, ""); err != nil {
		t.Fatalf("AddItem with grant: %v", err)
	}

	// A player without a grant gets 404 — the inventory's existence must not
	// leak (core domain rule 3).
	if _, err := env.inventory.ListInventory(ctx, otherPlayer, owner); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("ListInventory without grant = %v, want ErrNotFound", err)
	}
}

func TestInventoryService_LocationsHoldNoMoney(t *testing.T) {
	env := newOwnerInventoryTestEnv(t)
	ctx := t.Context()
	character := ownedCharacter(t, env.chars, "Aria")
	location := env.grantedLocation(t, character.ID)

	_, err := env.inventory.ListMoney(ctx, playerRequester, models.LocationOwner(location.ID))
	if !errors.Is(err, application.ErrInvalidOwner) {
		t.Fatalf("ListMoney(location) = %v, want ErrInvalidOwner", err)
	}
}

func TestInventoryService_MoveItem_FullMoveRetargets(t *testing.T) {
	env := newOwnerInventoryTestEnv(t)
	ctx := t.Context()
	character := ownedCharacter(t, env.chars, "Aria")
	group := env.memberGroup(t, character.ID)
	charOwner := models.CharacterOwner(character.ID)
	groupOwner := models.GroupOwner(group.ID)

	item, err := env.inventory.AddItem(ctx, playerRequester, charOwner, "Torch", nil, 3, "")
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	moved, err := env.inventory.MoveItem(ctx, playerRequester, item.ID, groupOwner, 3)
	if err != nil {
		t.Fatalf("MoveItem: %v", err)
	}
	if moved.GroupID == nil || *moved.GroupID != group.ID || moved.Quantity != 3 {
		t.Fatalf("moved = %+v, want 3 Torch owned by group %s", moved, group.ID)
	}

	source, err := env.inventory.ListInventory(ctx, playerRequester, charOwner)
	if err != nil {
		t.Fatalf("ListInventory(source): %v", err)
	}
	if len(source) != 0 {
		t.Fatalf("source still holds %d lines after full move, want 0", len(source))
	}
}

func TestInventoryService_MoveItem_PartialMoveSplitsAndMerges(t *testing.T) {
	env := newOwnerInventoryTestEnv(t)
	ctx := t.Context()
	character := ownedCharacter(t, env.chars, "Aria")
	location := env.grantedLocation(t, character.ID)
	charOwner := models.CharacterOwner(character.ID)
	locOwner := models.LocationOwner(location.ID)

	item, err := env.inventory.AddItem(ctx, playerRequester, charOwner, "Arrow", nil, 20, "")
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	// Split: 5 of 20 arrows go to the location.
	split, err := env.inventory.MoveItem(ctx, playerRequester, item.ID, locOwner, 5)
	if err != nil {
		t.Fatalf("MoveItem(split): %v", err)
	}
	if split.Quantity != 5 || split.LocationID == nil {
		t.Fatalf("split = %+v, want 5 at location", split)
	}

	remaining, err := env.inventory.ListInventory(ctx, playerRequester, charOwner)
	if err != nil {
		t.Fatalf("ListInventory: %v", err)
	}
	if len(remaining) != 1 || remaining[0].Quantity != 15 {
		t.Fatalf("source = %+v, want one line of 15", remaining)
	}

	// Merge: 5 more arrows join the existing location line instead of
	// creating a duplicate.
	merged, err := env.inventory.MoveItem(ctx, playerRequester, item.ID, locOwner, 5)
	if err != nil {
		t.Fatalf("MoveItem(merge): %v", err)
	}
	if merged.ID != split.ID || merged.Quantity != 10 {
		t.Fatalf("merged = %+v, want line %s at quantity 10", merged, split.ID)
	}

	atLocation, err := env.inventory.ListInventory(ctx, playerRequester, locOwner)
	if err != nil {
		t.Fatalf("ListInventory(location): %v", err)
	}
	if len(atLocation) != 1 {
		t.Fatalf("location holds %d lines, want 1 (merged)", len(atLocation))
	}
}

func TestInventoryService_MoveItem_Validation(t *testing.T) {
	env := newOwnerInventoryTestEnv(t)
	ctx := t.Context()
	character := ownedCharacter(t, env.chars, "Aria")
	charOwner := models.CharacterOwner(character.ID)

	item, err := env.inventory.AddItem(ctx, playerRequester, charOwner, "Torch", nil, 3, "")
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	if _, err := env.inventory.MoveItem(ctx, playerRequester, item.ID, charOwner, 1); !errors.Is(err, application.ErrSameInventory) {
		t.Fatalf("MoveItem(same inventory) = %v, want ErrSameInventory", err)
	}

	if _, err := env.inventory.MoveItem(ctx, playerRequester, item.ID, models.InventoryOwner{}, 1); !errors.Is(err, application.ErrInvalidOwner) {
		t.Fatalf("MoveItem(no target) = %v, want ErrInvalidOwner", err)
	}

	other := ownedCharacter(t, env.chars, "Beren")
	if _, err := env.inventory.MoveItem(ctx, playerRequester, item.ID, models.CharacterOwner(other.ID), 4); !errors.Is(err, application.ErrInvalidQuantity) {
		t.Fatalf("MoveItem(too many) = %v, want ErrInvalidQuantity", err)
	}
}

func TestInventoryService_MoveItem_AccessChecksBothEnds(t *testing.T) {
	env := newOwnerInventoryTestEnv(t)
	ctx := t.Context()
	character := ownedCharacter(t, env.chars, "Aria")
	group, err := env.groups.Create(ctx, gmRequester, "Sealed Order", models.GroupTypeOther, "")
	if err != nil {
		t.Fatalf("Create group: %v", err)
	}

	item, err := env.inventory.AddItem(
		ctx, playerRequester, models.CharacterOwner(character.ID), "Torch", nil, 3, "",
	)
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	// Owner of the item but not a member of the target group: 404 on target.
	_, err = env.inventory.MoveItem(ctx, playerRequester, item.ID, models.GroupOwner(group.ID), 1)
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("MoveItem(no target access) = %v, want ErrNotFound", err)
	}

	// A stranger must not even learn the item exists.
	otherChar, err := env.chars.Create(ctx, otherPlayer, "", "Sneak")
	if err != nil {
		t.Fatalf("Create character: %v", err)
	}
	_, err = env.inventory.MoveItem(ctx, otherPlayer, item.ID, models.CharacterOwner(otherChar.ID), 1)
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("MoveItem(foreign source) = %v, want ErrNotFound", err)
	}
}
