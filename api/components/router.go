package components

import (
	"github.com/DaanV2/itinerarium/api/handlers"
	"github.com/DaanV2/itinerarium/api/infrastructure/webapp"
	"github.com/DaanV2/itinerarium/api/transport"
	"github.com/charmbracelet/log"
)

// CreateRouter assembles the HTTP router. Public routes (health, setup, login)
// sit at the top; every authenticated resource is its own subrouter, mounted
// under an "authenticated" subrouter that applies RequireAuth once and is
// itself mounted under /api. Security middleware (headers + body-size cap)
// comes from the security config set (M10).
func CreateRouter(services *Services, logger *log.Logger) *transport.Router {
	// One shared login/reset limiter for the whole server (nil when disabled by
	// config). Login and reset use distinct key prefixes, so sharing is safe.
	loginThrottle := transport.NewLoginThrottle(LoginMaxFailuresFlag.Value(), LoginLockoutFlag.Value())
	trustProxy := TrustProxyHeadersFlag.Value()

	authenticated := transport.NewRouter(
		transport.WithMiddleware(transport.RequireAuth(services.Auth)),
		transport.WithSubRoute("/admin", adminRouter(services, loginThrottle)),
		transport.WithSubRoute("/characters", charactersRouter(services)),
		transport.WithSubRoute("/groups", groupsRouter(services)),
		transport.WithSubRoute("/sessions", sessionsRouter(services)),
		transport.WithSubRoute("/locations", locationsRouter(services)),
		transport.WithSubRoute("/currencies", currenciesRouter(services)),
		transport.WithSubRoute("/items", itemsRouter(services)),
		transport.WithSubRoute("/repositories", repositoriesRouter(services)),
		transport.WithSubRoute("/documents", documentsRouter(services)),
		transport.WithSubRoute("/activity", activityRouter(services)),
		transport.WithHandle("POST /inventory/move", handlers.MoveInventoryItemHandler(services.Inventory)),
		transport.WithHandle("GET /search", handlers.SearchDocumentsHandler(services.Documents)),
		transport.WithHandle("POST /import/obsidian", handlers.ImportVaultHandler(services.VaultImport)),
	)

	opts := []transport.Option{
		transport.WithMiddleware(transport.Logging(logger)),
		transport.WithMiddleware(transport.SecurityHeaders(CSPFlag.Value(), HSTSFlag.Value())),
		transport.WithMiddleware(transport.MaxBytes(int64(BodyLimitFlag.Value()))),
		transport.WithHandle("GET /api/health", transport.HealthHandler()),
		transport.WithHandle("GET /api/setup", handlers.SetupStatusHandler(services.Setup)),
		transport.WithHandle("POST /api/setup", handlers.CreateInitialGMHandler(services.Setup)),
		transport.WithHandle("POST /api/login", handlers.LoginHandler(services.Auth, loginThrottle, trustProxy)),
		transport.WithSubRoute("/api", authenticated),
	}

	// Everything outside /api serves the frontend embedded in the binary.
	// Builds without the embedweb tag (dev, plain `go build`) are API-only;
	// there the vite dev server hosts the frontend instead.
	if assets, ok := webapp.Assets(); ok {
		opts = append(opts, transport.WithHandle("/", transport.SPAHandler(assets)))
	} else {
		logger.Warn("built without the embedded web UI (embedweb build tag), serving the API only")
	}

	return transport.NewRouter(opts...)
}

// adminRouter serves account administration under /api/admin. The shared
// loginThrottle also caps password-reset spam per target account.
func adminRouter(services *Services, loginThrottle *transport.Throttle) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /users", handlers.ListAccountsHandler(services.Users)),
		transport.WithHandle("POST /users", handlers.CreateAccountHandler(services.Users)),
		transport.WithHandle(
			"POST /users/{id}/reset-password", handlers.ResetPasswordHandler(services.Users, loginThrottle),
		),
	)
}

// charactersRouter serves characters and their nested inventory/money
// subresources under /api/characters.
func charactersRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", handlers.ListCharactersHandler(services.Characters)),
		transport.WithHandle("POST /", handlers.CreateCharacterHandler(services.Characters)),
		transport.WithHandle("GET /{id}", handlers.GetCharacterHandler(services.Characters)),
		transport.WithHandle("PATCH /{id}", handlers.UpdateCharacterHandler(services.Characters)),
		transport.WithHandle("PUT /{id}/location", handlers.SetCharacterLocationHandler(services.Locations)),
		transport.WithHandle("DELETE /{id}/location", handlers.ClearCharacterLocationHandler(services.Locations)),
		transport.WithHandle("GET /{id}/activity", handlers.GetCharacterActivityHandler(services.Activity)),
		transport.WithSubRoute("/{id}/inventory", inventoryRouter(services, handlers.CharacterOwner)),
		transport.WithSubRoute("/{id}/money", moneyRouter(services, handlers.CharacterOwner)),
		transport.WithSubRoute("/{id}/journal", journalRouter(services)),
	)
}

// activityRouter serves the GM-wide campaign log and announcements under
// /api/activity. The per-character feed lives under
// /api/characters/{id}/activity.
func activityRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", handlers.ListActivityHandler(services.Activity)),
		transport.WithHandle("POST /announcements", handlers.AnnounceActivityHandler(services.Activity)),
	)
}

// journalRouter serves one character's journal entries under
// /api/characters/{id}/journal.
func journalRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", handlers.ListJournalEntriesHandler(services.Journals)),
		transport.WithHandle("POST /", handlers.CreateJournalEntryHandler(services.Journals)),
		transport.WithHandle("GET /{entryId}", handlers.GetJournalEntryHandler(services.Journals)),
		transport.WithHandle("PATCH /{entryId}", handlers.UpdateJournalEntryHandler(services.Journals)),
		transport.WithHandle("POST /{entryId}/convert", handlers.ConvertJournalEntryHandler(services.Journals)),
	)
}

// locationsRouter serves locations and their access grants under
// /api/locations.
func locationsRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", handlers.ListLocationsHandler(services.Locations)),
		transport.WithHandle("POST /", handlers.CreateLocationHandler(services.Locations)),
		transport.WithHandle("GET /{id}", handlers.GetLocationHandler(services.Locations)),
		transport.WithHandle("PATCH /{id}", handlers.UpdateLocationHandler(services.Locations)),
		transport.WithHandle("GET /{id}/access", handlers.ListLocationAccessHandler(services.Locations)),
		transport.WithHandle("POST /{id}/access", handlers.GrantLocationAccessHandler(services.Locations)),
		transport.WithHandle("DELETE /{id}/access/{accessId}", handlers.RevokeLocationAccessHandler(services.Locations)),
		transport.WithSubRoute("/{id}/inventory", inventoryRouter(services, handlers.LocationOwner)),
	)
}

// inventoryRouter serves one inventory (character, group, or location — the
// extractor decides what {id} names) under <resource>/{id}/inventory.
func inventoryRouter(services *Services, owner handlers.OwnerExtractor) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", handlers.ListInventoryHandler(services.Inventory, owner)),
		transport.WithHandle("POST /", handlers.AddInventoryItemHandler(services.Inventory, owner)),
		transport.WithHandle("PATCH /{itemId}", handlers.UpdateInventoryItemHandler(services.Inventory, owner)),
		transport.WithHandle("DELETE /{itemId}", handlers.RemoveInventoryItemHandler(services.Inventory, owner)),
	)
}

// moneyRouter serves one owner's balances (character or group) under
// <resource>/{id}/money.
func moneyRouter(services *Services, owner handlers.OwnerExtractor) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", handlers.ListMoneyHandler(services.Inventory, owner)),
		transport.WithHandle("PUT /{currencyId}", handlers.SetMoneyHandler(services.Inventory, owner)),
	)
}

// groupsRouter serves groups and membership under /api/groups.
func groupsRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", handlers.ListGroupsHandler(services.Groups)),
		transport.WithHandle("POST /", handlers.CreateGroupHandler(services.Groups)),
		transport.WithHandle("GET /{id}", handlers.GetGroupHandler(services.Groups)),
		transport.WithHandle("PATCH /{id}", handlers.UpdateGroupHandler(services.Groups)),
		transport.WithHandle("POST /{id}/members", handlers.JoinGroupHandler(services.Groups)),
		transport.WithHandle("DELETE /{id}/members/{characterId}", handlers.LeaveGroupHandler(services.Groups)),
		transport.WithSubRoute("/{id}/inventory", inventoryRouter(services, handlers.GroupOwner)),
		transport.WithSubRoute("/{id}/money", moneyRouter(services, handlers.GroupOwner)),
	)
}

// sessionsRouter serves sessions, participants, and game-day advances under
// /api/sessions.
func sessionsRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", handlers.ListSessionsHandler(services.Sessions)),
		transport.WithHandle("POST /", handlers.CreateSessionHandler(services.Sessions)),
		transport.WithHandle("GET /{id}", handlers.GetSessionHandler(services.Sessions)),
		transport.WithHandle("PATCH /{id}", handlers.UpdateSessionHandler(services.Sessions)),
		transport.WithHandle("POST /{id}/participants", handlers.AddSessionParticipantHandler(services.Sessions)),
		transport.WithHandle(
			"DELETE /{id}/participants/{characterId}", handlers.RemoveSessionParticipantHandler(services.Sessions),
		),
		transport.WithHandle("POST /{id}/game-day", handlers.AdvanceSessionGameDayHandler(services.Sessions)),
	)
}

// currenciesRouter serves the currency catalog under /api/currencies.
func currenciesRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", handlers.ListCurrenciesHandler(services.Catalog)),
		transport.WithHandle("POST /", handlers.CreateCurrencyHandler(services.Catalog)),
		transport.WithHandle("POST /convert", handlers.ConvertCurrencyHandler(services.Catalog)),
		transport.WithHandle("POST /simplify", handlers.SimplifyCurrencyHandler(services.Catalog)),
	)
}

// itemsRouter serves the item-definition catalog under /api/items.
func itemsRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", handlers.ListItemDefinitionsHandler(services.Catalog)),
		transport.WithHandle("POST /", handlers.CreateItemDefinitionHandler(services.Catalog)),
	)
}

// repositoriesRouter serves knowledge repositories (read-only — they are
// provisioned automatically, never created by a caller) under
// /api/repositories.
func repositoriesRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", handlers.ListRepositoriesHandler(services.Repositories)),
		transport.WithHandle("GET /{id}", handlers.GetRepositoryHandler(services.Repositories)),
		transport.WithHandle("GET /{id}/documents", handlers.ListDocumentsHandler(services.Documents)),
		transport.WithHandle("POST /{id}/documents", handlers.CreateDocumentHandler(services.Documents)),
		transport.WithHandle("GET /{id}/documents/tree", handlers.GetDocumentFolderTreeHandler(services.Documents)),
	)
}

// documentsRouter serves single documents under /api/documents. Documents
// are created through their repository; reads and edits address the document
// directly.
func documentsRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /shared", handlers.ListSharedDocumentsHandler(services.Documents)),
		transport.WithHandle("GET /{id}", handlers.GetDocumentHandler(services.Documents)),
		transport.WithHandle("PATCH /{id}", handlers.UpdateDocumentHandler(services.Documents)),
		transport.WithHandle("DELETE /{id}", handlers.DeleteDocumentHandler(services.Documents)),
		transport.WithHandle("POST /{id}/share", handlers.ShareDocumentHandler(services.Documents)),
		transport.WithHandle("GET /{id}/shares", handlers.ListDocumentSharesHandler(services.Documents)),
		transport.WithHandle("POST /{id}/shares", handlers.ShareDocumentWithCharacterHandler(services.Documents)),
		transport.WithHandle("DELETE /{id}/shares/{shareId}", handlers.RevokeDocumentShareHandler(services.Documents)),
	)
}
