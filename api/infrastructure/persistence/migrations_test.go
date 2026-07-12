package persistence_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
)

// TestMigrate_UpgradesM1InventorySchema proves an existing M1 database — where
// inventory_items.character_id and money_balances.character_id were NOT NULL —
// migrates to the M2 owner-based schema: legacy character rows survive, and
// group/location-owned rows (NULL character_id) become insertable.
func TestMigrate_UpgradesM1InventorySchema(t *testing.T) {
	db, err := persistence.New(persistence.WithInMemory())
	if err != nil {
		t.Fatalf("persistence.New: %v", err)
	}

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
		if err := db.DB().Exec(ddl).Error; err != nil {
			t.Fatalf("legacy DDL %q: %v", ddl, err)
		}
	}

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate over M1 schema: %v", err)
	}

	// Legacy character-owned rows survive with their owner intact.
	var legacy models.InventoryItem
	if err := db.DB().First(&legacy, "id = ?", "item-1").Error; err != nil {
		t.Fatalf("loading legacy item: %v", err)
	}
	if legacy.CharacterID == nil || *legacy.CharacterID != "char-1" {
		t.Fatalf("legacy item CharacterID = %v, want char-1", legacy.CharacterID)
	}

	// Group-owned rows (NULL character_id) are now insertable.
	groupItem := &models.InventoryItem{
		InventoryOwner: models.GroupOwner("group-1"),
		Name:           "Shared Rations",
		Quantity:       10,
	}
	if err := db.DB().Create(groupItem).Error; err != nil {
		t.Fatalf("inserting group-owned item after migration: %v", err)
	}

	groupBalance := &models.MoneyBalance{
		GroupID:    groupItem.GroupID,
		CurrencyID: "cur-1",
		Amount:     7,
	}
	if err := db.DB().Create(groupBalance).Error; err != nil {
		t.Fatalf("inserting group-owned balance after migration: %v", err)
	}
}
