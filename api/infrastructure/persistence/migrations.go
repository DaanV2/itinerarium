package persistence

import "github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"

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
		&models.Repository{},
		&models.Document{},
		&models.DocumentSection{},
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

	return d.db.AutoMigrate(list...)
}
