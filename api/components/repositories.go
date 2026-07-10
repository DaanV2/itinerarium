package components

import (
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// Repositories bundles every entity repository over a single database handle.
type Repositories struct {
	Users           *repositories.Users
	Characters      *repositories.Characters
	Locations       *repositories.Locations
	Currencies      *repositories.Currencies
	ItemDefinitions *repositories.ItemDefinitions
	InventoryItems  *repositories.InventoryItems
	MoneyBalances   *repositories.MoneyBalances
	RevokedTokens   *repositories.RevokedTokens
}

// NewRepositories constructs every repository against the given database.
func NewRepositories(db *persistence.Database) *Repositories {
	return &Repositories{
		Users:           repositories.NewUsers(db),
		Characters:      repositories.NewCharacters(db),
		Locations:       repositories.NewLocations(db),
		Currencies:      repositories.NewCurrencies(db),
		ItemDefinitions: repositories.NewItemDefinitions(db),
		InventoryItems:  repositories.NewInventoryItems(db),
		MoneyBalances:   repositories.NewMoneyBalances(db),
		RevokedTokens:   repositories.NewRevokedTokens(db),
	}
}
