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
		transport.WithSubRoute("/currencies", currenciesRouter(services)),
		transport.WithSubRoute("/items", itemsRouter(services)),
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
		transport.WithSubRoute("/{id}/inventory", inventoryRouter(services)),
		transport.WithSubRoute("/{id}/money", moneyRouter(services)),
	)
}

// inventoryRouter serves a character's inventory under
// /api/characters/{id}/inventory.
func inventoryRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", transport.ListInventoryHandler(services.Inventory)),
		transport.WithHandle("POST /", transport.AddInventoryItemHandler(services.Inventory)),
		transport.WithHandle("PATCH /{itemId}", transport.UpdateInventoryItemHandler(services.Inventory)),
		transport.WithHandle("DELETE /{itemId}", transport.RemoveInventoryItemHandler(services.Inventory)),
	)
}

// moneyRouter serves a character's balances under /api/characters/{id}/money.
func moneyRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", transport.ListMoneyHandler(services.Inventory)),
		transport.WithHandle("PUT /{currencyId}", transport.SetMoneyHandler(services.Inventory)),
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
