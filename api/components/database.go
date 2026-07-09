package components

import (
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
)

// SetupDatabase opens the SQLite database at the configured path and applies
// migrations. The returned *persistence.Database participates in the shutdown
// lifecycle.
func SetupDatabase(cfg *ServerConfig) (*persistence.Database, error) {
	db, err := persistence.New(persistence.WithPath(cfg.DatabasePath))
	if err != nil {
		return nil, err
	}

	if err := db.Migrate(); err != nil {
		return nil, err
	}

	return db, nil
}
