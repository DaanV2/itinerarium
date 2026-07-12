package components

import (
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// Repositories bundles every entity repository over a single database handle.
type Repositories struct {
	Users                 *repositories.Users
	Characters            *repositories.Characters
	Currencies            *repositories.Currencies
	ItemDefinitions       *repositories.ItemDefinitions
	InventoryItems        *repositories.InventoryItems
	MoneyBalances         *repositories.MoneyBalances
	Groups                *repositories.Groups
	ActivityEntries       *repositories.ActivityEntries
	Locations             *repositories.Locations
	LocationAccesses      *repositories.LocationAccesses
	RevokedTokens         *repositories.RevokedTokens
	KnowledgeRepositories *repositories.KnowledgeRepositories
}

// NewRepositories constructs every repository against the given database.
func NewRepositories(db *persistence.Database) *Repositories {
	return &Repositories{
		Users:                 repositories.NewUsers(db),
		Characters:            repositories.NewCharacters(db),
		Currencies:            repositories.NewCurrencies(db),
		ItemDefinitions:       repositories.NewItemDefinitions(db),
		InventoryItems:        repositories.NewInventoryItems(db),
		MoneyBalances:         repositories.NewMoneyBalances(db),
		Groups:                repositories.NewGroups(db),
		ActivityEntries:       repositories.NewActivityEntries(db),
		Locations:             repositories.NewLocations(db),
		LocationAccesses:      repositories.NewLocationAccesses(db),
		RevokedTokens:         repositories.NewRevokedTokens(db),
		KnowledgeRepositories: repositories.NewKnowledgeRepositories(db),
	}
}
