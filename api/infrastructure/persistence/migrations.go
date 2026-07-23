package persistence

import (
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// allModels returns every GORM model, in dependency order. Register each new
// model here — the baseline migration (0001) AutoMigrates exactly this list, so
// a model missing from it gets no table on a fresh install.
func allModels() []any {
	return []any{
		&models.User{},
		&models.RevokedToken{},
		&models.Location{},
		&models.LocationSection{},
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

// migrations is the ordered, append-only schema history (M11). gormigrate
// records each applied ID in a "migrations" table and runs only the pending
// ones, so in-place upgrades are deterministic across restarts and backends.
//
// There is deliberately no InitSchema shortcut: the list runs on every
// database, so a deployment that predates gormigrate (schema present, no
// "migrations" table) still runs 0002 when it first adopts tracking, instead of
// having every migration silently marked applied. Both migrations here are
// idempotent, so running them against an already-current schema is a no-op.
//
// RULES FOR FUTURE CHANGES:
//   - 0001 intentionally tracks allModels() as the fresh-install baseline; it is
//     the only AutoMigrate fast path. Do NOT edit it to alter an existing
//     deployment's schema — AutoMigrate never drops or renames columns and can't
//     express a data backfill.
//   - Every schema change from now on is a NEW numbered migration appended here
//     (0003, 0004, …). Adding a model to allModels() alone only reaches fresh
//     installs; existing deployments need the numbered migration to get the table.
func migrations() []*gormigrate.Migration {
	return []*gormigrate.Migration{
		{
			// 0001 — the full current schema. On a fresh database this creates
			// every table; on an existing one it is an idempotent no-op.
			ID: "0001_initial_schema",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(allModels()...)
			},
		},
		{
			// 0002 — stamp the M5 access-scope columns onto activity entries
			// written before those columns existed. Pre-M5 the only recorded
			// events were group membership changes, whose entity doubles as their
			// scope. Idempotent: it only touches rows still missing a scope.
			ID: "0002_backfill_activity_scopes",
			Migrate: func(tx *gorm.DB) error {
				return tx.Model(&models.ActivityEntry{}).
					Where("(scope_type IS NULL OR scope_type = '') AND entity_type = ?", models.ActivityScopeGroup).
					Updates(map[string]any{
						"scope_type": models.ActivityScopeGroup,
						"scope_id":   gorm.Expr("entity_id"),
					}).Error
			},
		},
	}
}

// Migrate brings the schema up to date by running every pending migration in
// order (M11). Both `serve` and `init` reach this through
// components.SetupDatabase, so an upgrade applies automatically on start.
func (d *Database) Migrate() error {
	m := gormigrate.New(d.db, gormigrate.DefaultOptions, migrations())

	return m.Migrate()
}
