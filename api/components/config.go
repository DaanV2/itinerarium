package components

import (
	"github.com/DaanV2/itinerarium/api/infrastructure/config"
)

// CatalogConfigSet groups the catalog seeding flags, consumed by seedCatalog
// in BuildServer. Commands opt in with AddToSet.
var (
	CatalogConfigSet = config.New("catalog")

	CatalogPathFlag = CatalogConfigSet.String("catalog.path", "",
		"optional JSON/YAML file seeding the currency and item catalog on startup")
)
