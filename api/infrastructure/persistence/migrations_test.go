package persistence_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
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
