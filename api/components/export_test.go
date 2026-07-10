package components

import "github.com/DaanV2/itinerarium/api/infrastructure/persistence"

// DatabaseOptions exposes the unexported backend-selection logic to the
// external test package.
func DatabaseOptions(cfg DatabaseConfig) ([]persistence.Option, error) {
	return databaseOptions(cfg)
}
