package components

import (
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
)

// SetupDatabase opens the database backend configured by the "database" flags
// and applies migrations. The returned *persistence.Database participates in
// the shutdown lifecycle.
func SetupDatabase() (*persistence.Database, error) {
	opts, err := persistence.GetOptions()
	if err != nil {
		return nil, err
	}

	db, err := persistence.New(opts...)
	if err != nil {
		return nil, err
	}

	if err := db.Migrate(); err != nil {
		return nil, err
	}

	return db, nil
}
