package persistence_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrate_UpgradesM1InventorySchema proves an existing M1 database — where
// inventory_items.character_id and money_balances.character_id were NOT NULL —
// migrates to the M2 owner-based schema: legacy character rows survive, and
// group/location-owned rows (NULL character_id) become insertable.
func TestMigrate_UpgradesM1InventorySchema(t *testing.T) {
	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err, "persistence.New")

	// Recreate the M1 tables as AutoMigrate built them before this change.
	legacyDDL := []string{
		"CREATE TABLE `inventory_items` (`id` uuid,`created_at` datetime,`updated_at` datetime," +
			"`deleted_at` datetime,`character_id` uuid NOT NULL,`name` text NOT NULL," +
			"`item_definition_id` uuid,`quantity` integer NOT NULL DEFAULT 1,`description` text," +
			"PRIMARY KEY (`id`))",
		"CREATE TABLE `money_balances` (`id` uuid,`created_at` datetime,`updated_at` datetime," +
			"`deleted_at` datetime,`character_id` uuid NOT NULL,`currency_id` uuid NOT NULL," +
			"`amount` integer NOT NULL DEFAULT 0,PRIMARY KEY (`id`))",
		"CREATE UNIQUE INDEX `idx_money_character_currency` ON `money_balances`(`character_id`,`currency_id`)",
		"INSERT INTO `inventory_items` (`id`,`character_id`,`name`,`quantity`) " +
			"VALUES ('item-1','char-1','Torch',3)",
		"INSERT INTO `money_balances` (`id`,`character_id`,`currency_id`,`amount`) " +
			"VALUES ('bal-1','char-1','cur-1',42)",
	}
	for _, ddl := range legacyDDL {
		require.NoError(t, db.DB().Exec(ddl).Error, "legacy DDL %q", ddl)
	}

	require.NoError(t, db.Migrate(), "Migrate over M1 schema")

	// Legacy character-owned rows survive with their owner intact.
	var legacy models.InventoryItem
	require.NoError(t, db.DB().First(&legacy, "id = ?", "item-1").Error, "loading legacy item")
	require.NotNil(t, legacy.CharacterID)
	require.Equal(t, "char-1", *legacy.CharacterID)

	// Group-owned rows (NULL character_id) are now insertable.
	groupItem := &models.InventoryItem{
		InventoryOwner: models.GroupOwner("group-1"),
		Name:           "Shared Rations",
		Quantity:       10,
	}
	require.NoError(t, db.DB().Create(groupItem).Error, "inserting group-owned item after migration")

	groupBalance := &models.MoneyBalance{
		GroupID:    groupItem.GroupID,
		CurrencyID: "cur-1",
		Amount:     7,
	}
	require.NoError(t, db.DB().Create(groupBalance).Error, "inserting group-owned balance after migration")
}

func TestMigrate_FreshDatabaseCreatesEverySchema(t *testing.T) {
	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err)

	require.NoError(t, db.Migrate())

	migrator := db.DB().Migrator()
	for _, model := range []any{
		&models.User{}, &models.RevokedToken{}, &models.Location{}, &models.Character{},
		&models.Group{}, &models.ActivityEntry{}, &models.Repository{}, &models.Document{},
		&models.JournalEntry{}, &models.Session{},
	} {
		assert.Truef(t, migrator.HasTable(model), "expected a table for %T", model)
	}

	// gormigrate records every applied migration in the "migrations" table.
	var applied int64
	require.NoError(t, db.DB().Table("migrations").Count(&applied).Error)
	assert.Equal(t, int64(2), applied, "both numbered migrations should be recorded")
}

func TestMigrate_IsIdempotent(t *testing.T) {
	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err)

	require.NoError(t, db.Migrate())
	require.NoError(t, db.Migrate(), "a second Migrate on an up-to-date database must be a clean no-op")

	var applied int64
	require.NoError(t, db.DB().Table("migrations").Count(&applied).Error)
	assert.Equal(t, int64(2), applied, "re-running must not duplicate migration records")
}

// TestMigrate_BackfillsActivityScopesOnAdoption simulates a deployment that
// predates gormigrate: the schema exists but there is no "migrations" table and
// a group activity entry still lacks its access scope. Migrate must run the
// backfill (0002) rather than silently marking it applied.
func TestMigrate_BackfillsActivityScopesOnAdoption(t *testing.T) {
	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err)

	// Old-world schema: created straight from the model, no migration history.
	require.NoError(t, db.DB().AutoMigrate(&models.ActivityEntry{}))

	entry := &models.ActivityEntry{
		GameDay:    3,
		Action:     models.ActivityActionJoined,
		EntityType: models.ActivityScopeGroup,
		EntityID:   "11111111-1111-1111-1111-111111111111",
		EntityName: "The Thieves Guild",
		// ScopeType/ScopeID intentionally empty — the pre-M5 state 0002 backfills.
	}
	require.NoError(t, db.DB().Create(entry).Error)

	require.NoError(t, db.Migrate())

	var got models.ActivityEntry
	require.NoError(t, db.DB().First(&got, "id = ?", entry.ID).Error)
	assert.Equal(t, models.ActivityScopeGroup, got.ScopeType, "scope type should be backfilled from the group entity")
	assert.Equal(t, got.EntityID, got.ScopeID, "scope id should be backfilled from the entity id")
}
