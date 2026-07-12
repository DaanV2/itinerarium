package components

import (
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
	"github.com/charmbracelet/log"
)

// CreateRouter assembles the HTTP router. Public routes (health, setup, login)
// sit at the top; every authenticated resource is its own subrouter, mounted
// under an "authenticated" subrouter that applies RequireAuth once and is
// itself mounted under /api.
func CreateRouter(services *Services, logger *log.Logger) *transport.Router {
	authenticated := transport.NewRouter(
		transport.WithMiddleware(transport.RequireAuth(services.Auth)),
		transport.WithSubRoute("/admin", adminRouter(services)),
		transport.WithSubRoute("/characters", charactersRouter(services)),
		transport.WithSubRoute("/groups", groupsRouter(services)),
		transport.WithSubRoute("/locations", locationsRouter(services)),
		transport.WithSubRoute("/currencies", currenciesRouter(services)),
		transport.WithSubRoute("/items", itemsRouter(services)),
		transport.WithSubRoute("/repositories", repositoriesRouter(services)),
		transport.WithHandle("POST /inventory/move", transport.MoveInventoryItemHandler(services.Inventory)),
	)

	return transport.NewRouter(
		transport.WithMiddleware(transport.Logging(logger)),
		transport.WithHandle("GET /api/health", transport.HealthHandler()),
		transport.WithHandle("GET /api/setup", transport.SetupStatusHandler(services.Setup)),
		transport.WithHandle("POST /api/setup", transport.CreateInitialGMHandler(services.Setup)),
		transport.WithHandle("POST /api/login", transport.LoginHandler(services.Auth)),
		transport.WithSubRoute("/api", authenticated),
	)
}

// adminRouter serves account administration under /api/admin.
func adminRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /users", transport.ListAccountsHandler(services.Users)),
		transport.WithHandle("POST /users", transport.CreateAccountHandler(services.Users)),
		transport.WithHandle("POST /users/{id}/reset-password", transport.ResetPasswordHandler(services.Users)),
	)
}

// charactersRouter serves characters and their nested inventory/money
// subresources under /api/characters.
func charactersRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", transport.ListCharactersHandler(services.Characters)),
		transport.WithHandle("POST /", transport.CreateCharacterHandler(services.Characters)),
		transport.WithHandle("GET /{id}", transport.GetCharacterHandler(services.Characters)),
		transport.WithHandle("PATCH /{id}", transport.UpdateCharacterHandler(services.Characters)),
		transport.WithHandle("PUT /{id}/location", transport.SetCharacterLocationHandler(services.Locations)),
		transport.WithHandle("DELETE /{id}/location", transport.ClearCharacterLocationHandler(services.Locations)),
		transport.WithSubRoute("/{id}/inventory", inventoryRouter(services, transport.CharacterOwner)),
		transport.WithSubRoute("/{id}/money", moneyRouter(services, transport.CharacterOwner)),
	)
}

// locationsRouter serves locations and their access grants under
// /api/locations.
func locationsRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", transport.ListLocationsHandler(services.Locations)),
		transport.WithHandle("POST /", transport.CreateLocationHandler(services.Locations)),
		transport.WithHandle("GET /{id}", transport.GetLocationHandler(services.Locations)),
		transport.WithHandle("PATCH /{id}", transport.UpdateLocationHandler(services.Locations)),
		transport.WithHandle("GET /{id}/access", transport.ListLocationAccessHandler(services.Locations)),
		transport.WithHandle("POST /{id}/access", transport.GrantLocationAccessHandler(services.Locations)),
		transport.WithHandle("DELETE /{id}/access/{accessId}", transport.RevokeLocationAccessHandler(services.Locations)),
		transport.WithSubRoute("/{id}/inventory", inventoryRouter(services, transport.LocationOwner)),
	)
}

// inventoryRouter serves one inventory (character, group, or location — the
// extractor decides what {id} names) under <resource>/{id}/inventory.
func inventoryRouter(services *Services, owner transport.OwnerExtractor) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", transport.ListInventoryHandler(services.Inventory, owner)),
		transport.WithHandle("POST /", transport.AddInventoryItemHandler(services.Inventory, owner)),
		transport.WithHandle("PATCH /{itemId}", transport.UpdateInventoryItemHandler(services.Inventory, owner)),
		transport.WithHandle("DELETE /{itemId}", transport.RemoveInventoryItemHandler(services.Inventory, owner)),
	)
}

// moneyRouter serves one owner's balances (character or group) under
// <resource>/{id}/money.
func moneyRouter(services *Services, owner transport.OwnerExtractor) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", transport.ListMoneyHandler(services.Inventory, owner)),
		transport.WithHandle("PUT /{currencyId}", transport.SetMoneyHandler(services.Inventory, owner)),
	)
}

// groupsRouter serves groups and membership under /api/groups.
func groupsRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", transport.ListGroupsHandler(services.Groups)),
		transport.WithHandle("POST /", transport.CreateGroupHandler(services.Groups)),
		transport.WithHandle("GET /{id}", transport.GetGroupHandler(services.Groups)),
		transport.WithHandle("PATCH /{id}", transport.UpdateGroupHandler(services.Groups)),
		transport.WithHandle("POST /{id}/members", transport.JoinGroupHandler(services.Groups)),
		transport.WithHandle("DELETE /{id}/members/{characterId}", transport.LeaveGroupHandler(services.Groups)),
		transport.WithSubRoute("/{id}/inventory", inventoryRouter(services, transport.GroupOwner)),
		transport.WithSubRoute("/{id}/money", moneyRouter(services, transport.GroupOwner)),
	)
}

// currenciesRouter serves the currency catalog under /api/currencies.
func currenciesRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", transport.ListCurrenciesHandler(services.Catalog)),
		transport.WithHandle("POST /", transport.CreateCurrencyHandler(services.Catalog)),
	)
}

// itemsRouter serves the item-definition catalog under /api/items.
func itemsRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", transport.ListItemDefinitionsHandler(services.Catalog)),
		transport.WithHandle("POST /", transport.CreateItemDefinitionHandler(services.Catalog)),
	)
}

// repositoriesRouter serves knowledge repositories (read-only — they are
// provisioned automatically, never created by a caller) under
// /api/repositories.
func repositoriesRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", transport.ListRepositoriesHandler(services.Repositories)),
		transport.WithHandle("GET /{id}", transport.GetRepositoryHandler(services.Repositories)),
	)
}
