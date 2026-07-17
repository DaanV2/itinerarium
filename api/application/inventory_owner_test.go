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
	require.NoError(t, err)
	err = db.Migrate()
	require.NoError(t, err)

	characters := repositories.NewCharacters(db)
	groups := repositories.NewGroups(db)
	currencies := repositories.NewCurrencies(db)
	itemDefs := repositories.NewItemDefinitions(db)
	knowledgeRepo := repositories.NewKnowledgeRepositories(db)
	charSvc := application.NewCharacterService(characters, repositories.NewUsers(db), knowledgeRepo)
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
		groups:    application.NewGroupService(groups, charSvc, knowledgeRepo),
		locations: locationSvc,
	}
}

// memberGroup creates a group and joins the given character to it.
func (e ownerInventoryTestEnv) memberGroup(t *testing.T, characterID string) *models.Group {
	t.Helper()

	group, err := e.groups.Create(t.Context(), gmRequester, "Thieves Guild", models.GroupTypeOrganization, "")
	require.NoError(t, err)
	err = e.groups.Join(t.Context(), gmRequester, group.ID, characterID)
	require.NoError(t, err)

	return group
}

// grantedLocation creates a location and grants the given character access.
func (e ownerInventoryTestEnv) grantedLocation(t *testing.T, characterID string) *models.Location {
	t.Helper()

	location, err := e.locations.Create(t.Context(), gmRequester, "The Vault", "", "Material")
	require.NoError(t, err)
	_, err = e.locations.GrantAccess(t.Context(), gmRequester, location.ID, &characterID, nil)
	require.NoError(t, err)

	return location
}

func TestInventoryService_GroupInventory_MembersOnly(t *testing.T) {
	env := newOwnerInventoryTestEnv(t)
	ctx := t.Context()
	character := ownedCharacter(t, env.chars, "Aria")
	group := env.memberGroup(t, character.ID)
	owner := models.GroupOwner(group.ID)

	_, err := env.inventory.AddItem(ctx, playerRequester, owner, "Shared Rations", nil, 10, "")
	require.NoError(t, err)

	items, err := env.inventory.ListInventory(ctx, playerRequester, owner)
	require.NoError(t, err)
	assert.Len(t, items, 1)

	// A player without a member character gets 404 — the shared inventory is
	// member-only content even though the group itself is public.
	_, err = env.inventory.ListInventory(ctx, otherPlayer, owner)
	require.ErrorIs(t, err, application.ErrNotFound)
	_, err = env.inventory.AddItem(ctx, otherPlayer, owner, "Loot", nil, 1, "")
	require.ErrorIs(t, err, application.ErrNotFound)

	// GMs always have access.
	_, err = env.inventory.ListInventory(ctx, gmRequester, owner)
	require.NoError(t, err)
}

func TestInventoryService_GroupMoney_MembersOnly(t *testing.T) {
	env := newOwnerInventoryTestEnv(t)
	ctx := t.Context()
	character := ownedCharacter(t, env.chars, "Aria")
	group := env.memberGroup(t, character.ID)
	owner := models.GroupOwner(group.ID)

	gp, err := env.catalog.CreateCurrency(ctx, gmRequester, "gp", "Gold", 100)
	require.NoError(t, err)

	_, err = env.inventory.SetMoney(ctx, playerRequester, owner, gp.ID, 250)
	require.NoError(t, err)

	balances, err := env.inventory.ListMoney(ctx, playerRequester, owner)
	require.NoError(t, err)
	if assert.Len(t, balances, 1) {
		assert.Equal(t, int64(250), balances[0].Amount)
	}

	_, err = env.inventory.ListMoney(ctx, otherPlayer, owner)
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestInventoryService_LocationInventory_GrantGated(t *testing.T) {
	env := newOwnerInventoryTestEnv(t)
	ctx := t.Context()
	character := ownedCharacter(t, env.chars, "Aria")
	location := env.grantedLocation(t, character.ID)
	owner := models.LocationOwner(location.ID)

	_, err := env.inventory.AddItem(ctx, playerRequester, owner, "Stored Gear", nil, 2, "")
	require.NoError(t, err)

	// A player without a grant gets 404 — the inventory's existence must not
	// leak (core domain rule 3).
	_, err = env.inventory.ListInventory(ctx, otherPlayer, owner)
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestInventoryService_LocationsHoldNoMoney(t *testing.T) {
	env := newOwnerInventoryTestEnv(t)
	ctx := t.Context()
	character := ownedCharacter(t, env.chars, "Aria")
	location := env.grantedLocation(t, character.ID)

	_, err := env.inventory.ListMoney(ctx, playerRequester, models.LocationOwner(location.ID))
	require.ErrorIs(t, err, application.ErrInvalidOwner)
}

func TestInventoryService_MoveItem_FullMoveRetargets(t *testing.T) {
	env := newOwnerInventoryTestEnv(t)
	ctx := t.Context()
	character := ownedCharacter(t, env.chars, "Aria")
	group := env.memberGroup(t, character.ID)
	charOwner := models.CharacterOwner(character.ID)
	groupOwner := models.GroupOwner(group.ID)

	item, err := env.inventory.AddItem(ctx, playerRequester, charOwner, "Torch", nil, 3, "")
	require.NoError(t, err)

	moved, err := env.inventory.MoveItem(ctx, playerRequester, item.ID, groupOwner, 3)
	require.NoError(t, err)
	if assert.NotNil(t, moved.GroupID) {
		assert.Equal(t, group.ID, *moved.GroupID)
	}
	assert.Equal(t, 3, moved.Quantity)

	source, err := env.inventory.ListInventory(ctx, playerRequester, charOwner)
	require.NoError(t, err)
	assert.Empty(t, source, "source still holds lines after full move")
}

func TestInventoryService_MoveItem_PartialMoveSplitsAndMerges(t *testing.T) {
	env := newOwnerInventoryTestEnv(t)
	ctx := t.Context()
	character := ownedCharacter(t, env.chars, "Aria")
	location := env.grantedLocation(t, character.ID)
	charOwner := models.CharacterOwner(character.ID)
	locOwner := models.LocationOwner(location.ID)

	item, err := env.inventory.AddItem(ctx, playerRequester, charOwner, "Arrow", nil, 20, "")
	require.NoError(t, err)

	// Split: 5 of 20 arrows go to the location.
	split, err := env.inventory.MoveItem(ctx, playerRequester, item.ID, locOwner, 5)
	require.NoError(t, err)
	assert.Equal(t, 5, split.Quantity)
	assert.NotNil(t, split.LocationID)

	remaining, err := env.inventory.ListInventory(ctx, playerRequester, charOwner)
	require.NoError(t, err)
	if assert.Len(t, remaining, 1) {
		assert.Equal(t, 15, remaining[0].Quantity)
	}

	// Merge: 5 more arrows join the existing location line instead of
	// creating a duplicate.
	merged, err := env.inventory.MoveItem(ctx, playerRequester, item.ID, locOwner, 5)
	require.NoError(t, err)
	assert.Equal(t, split.ID, merged.ID)
	assert.Equal(t, 10, merged.Quantity)

	atLocation, err := env.inventory.ListInventory(ctx, playerRequester, locOwner)
	require.NoError(t, err)
	assert.Len(t, atLocation, 1, "location holds lines, want 1 (merged)")
}

func TestInventoryService_MoveItem_Validation(t *testing.T) {
	env := newOwnerInventoryTestEnv(t)
	ctx := t.Context()
	character := ownedCharacter(t, env.chars, "Aria")
	charOwner := models.CharacterOwner(character.ID)

	item, err := env.inventory.AddItem(ctx, playerRequester, charOwner, "Torch", nil, 3, "")
	require.NoError(t, err)

	_, err = env.inventory.MoveItem(ctx, playerRequester, item.ID, charOwner, 1)
	require.ErrorIs(t, err, application.ErrSameInventory)

	_, err = env.inventory.MoveItem(ctx, playerRequester, item.ID, models.InventoryOwner{}, 1)
	require.ErrorIs(t, err, application.ErrInvalidOwner)

	other := ownedCharacter(t, env.chars, "Beren")
	_, err = env.inventory.MoveItem(ctx, playerRequester, item.ID, models.CharacterOwner(other.ID), 4)
	require.ErrorIs(t, err, application.ErrInvalidQuantity)
}

func TestInventoryService_MoveItem_AccessChecksBothEnds(t *testing.T) {
	env := newOwnerInventoryTestEnv(t)
	ctx := t.Context()
	character := ownedCharacter(t, env.chars, "Aria")
	group, err := env.groups.Create(ctx, gmRequester, "Sealed Order", models.GroupTypeOther, "")
	require.NoError(t, err)

	item, err := env.inventory.AddItem(
		ctx, playerRequester, models.CharacterOwner(character.ID), "Torch", nil, 3, "",
	)
	require.NoError(t, err)

	// Owner of the item but not a member of the target group: 404 on target.
	_, err = env.inventory.MoveItem(ctx, playerRequester, item.ID, models.GroupOwner(group.ID), 1)
	require.ErrorIs(t, err, application.ErrNotFound)

	// A stranger must not even learn the item exists.
	otherChar, err := env.chars.Create(ctx, otherPlayer, "", "Sneak")
	require.NoError(t, err)
	_, err = env.inventory.MoveItem(ctx, otherPlayer, item.ID, models.CharacterOwner(otherChar.ID), 1)
	require.ErrorIs(t, err, application.ErrNotFound)
}
