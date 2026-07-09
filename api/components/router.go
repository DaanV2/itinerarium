package components

import (
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
	"github.com/charmbracelet/log"
)

// CreateRouter assembles the HTTP router: request logging middleware, the
// public setup/login routes, and an auth-gated subrouter for the rest of the
// API. Handlers are thin adapters over the application services.
func CreateRouter(services *Services, logger *log.Logger) *transport.Router {
	return transport.NewRouter(
		transport.WithMiddleware(transport.Logging(logger)),

		// Public
		transport.WithHandle("GET /api/health", transport.HealthHandler()),
		transport.WithHandle("GET /api/setup", transport.SetupStatusHandler(services.Setup)),
		transport.WithHandle("POST /api/setup", transport.CreateInitialGMHandler(services.Setup)),
		transport.WithHandle("POST /api/login", transport.LoginHandler(services.Auth)),

		// Authenticated — RequireAuth wraps every route in the group once.
		transport.WithGroup(transport.RequireAuth(services.Auth),
			// Admin
			transport.WithHandle("GET /api/admin/users", transport.ListAccountsHandler(services.Users)),
			transport.WithHandle("POST /api/admin/users", transport.CreateAccountHandler(services.Users)),
			transport.WithHandle(
				"POST /api/admin/users/{id}/reset-password",
				transport.ResetPasswordHandler(services.Users),
			),

			// Characters
			transport.WithHandle("GET /api/characters", transport.ListCharactersHandler(services.Characters)),
			transport.WithHandle("POST /api/characters", transport.CreateCharacterHandler(services.Characters)),
			transport.WithHandle("GET /api/characters/{id}", transport.GetCharacterHandler(services.Characters)),
			transport.WithHandle("PATCH /api/characters/{id}", transport.UpdateCharacterHandler(services.Characters)),

			// Catalog
			transport.WithHandle("GET /api/currencies", transport.ListCurrenciesHandler(services.Catalog)),
			transport.WithHandle("POST /api/currencies", transport.CreateCurrencyHandler(services.Catalog)),
			transport.WithHandle("GET /api/items", transport.ListItemDefinitionsHandler(services.Catalog)),
			transport.WithHandle("POST /api/items", transport.CreateItemDefinitionHandler(services.Catalog)),

			// Inventory
			transport.WithHandle(
				"GET /api/characters/{id}/inventory",
				transport.ListInventoryHandler(services.Inventory),
			),
			transport.WithHandle(
				"POST /api/characters/{id}/inventory",
				transport.AddInventoryItemHandler(services.Inventory),
			),
			transport.WithHandle(
				"PATCH /api/characters/{id}/inventory/{itemId}",
				transport.UpdateInventoryItemHandler(services.Inventory),
			),
			transport.WithHandle(
				"DELETE /api/characters/{id}/inventory/{itemId}",
				transport.RemoveInventoryItemHandler(services.Inventory),
			),

			// Money
			transport.WithHandle("GET /api/characters/{id}/money", transport.ListMoneyHandler(services.Inventory)),
			transport.WithHandle(
				"PUT /api/characters/{id}/money/{currencyId}",
				transport.SetMoneyHandler(services.Inventory),
			),
		),
	)
}
