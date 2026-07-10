package components

import (
	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
)

// Services bundles every application-layer service. Services own the business
// logic and permission rules; the transport layer calls into them.
type Services struct {
	Setup      *application.SetupService
	Auth       *application.AuthService
	Users      *application.UserService
	Characters *application.CharacterService
	Locations  *application.LocationService
	Catalog    *application.CatalogService
	Inventory  *application.InventoryService
	Groups     *application.GroupService
}

// NewServices wires the application services over the repositories and token
// service.
func NewServices(repos *Repositories, tokens *authentication.TokenService) *Services {
	characters := application.NewCharacterService(repos.Characters, repos.Users, repos.Locations)

	return &Services{
		Setup:      application.NewSetupService(repos.Users, tokens),
		Auth:       application.NewAuthService(tokens, repos.Users),
		Users:      application.NewUserService(repos.Users),
		Characters: characters,
		Locations:  application.NewLocationService(repos.Locations),
		Catalog:    application.NewCatalogService(repos.Currencies, repos.ItemDefinitions),
		Inventory: application.NewInventoryService(
			characters,
			repos.InventoryItems,
			repos.MoneyBalances,
			repos.Currencies,
			repos.ItemDefinitions,
		),
		Groups: application.NewGroupService(repos.Groups, characters),
	}
}
