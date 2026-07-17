package persistence

import (
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
)

// allModels returns every GORM model, in dependency order. Register each new
// model here — AutoMigrate only creates tables for models in this list.
func allModels() []any {
	return []any{
		&models.User{},
		&models.RevokedToken{},
		&models.Location{},
		&models.LocationAccess{},
		&models.Character{},
		&models.Currency{},
		&models.ItemDefinition{},
		&models.InventoryItem{},
		&models.MoneyBalance{},
		&models.Group{},
		&models.ActivityEntry{},
		&models.ActivityTarget{},
		&models.Repository{},
		&models.Document{},
		&models.DocumentSection{},
		&models.DocumentShare{},
		&models.JournalEntry{},
		&models.Session{},
	}
}

// Migrate brings the schema up to date for every registered model.
func (d *Database) Migrate() error {
	list := allModels()
	if len(list) == 0 {
		return nil
	}

	if err := d.db.AutoMigrate(list...); err != nil {
		return err
	}

	return d.backfillActivityScopes()
}

// backfillActivityScopes stamps the M5 access-scope columns onto activity
// entries written before those columns existed. Pre-M5 the only recorded
// events were group membership changes, whose entity doubles as their scope.
func (d *Database) backfillActivityScopes() error {
	return d.db.Model(&models.ActivityEntry{}).
		Where("(scope_type IS NULL OR scope_type = '') AND entity_type = ?", models.ActivityScopeGroup).
		Updates(map[string]any{"scope_type": models.ActivityScopeGroup, "scope_id": gorm.Expr("entity_id")}).Error
}
