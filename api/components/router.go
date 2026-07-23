package components

import (
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
	"github.com/DaanV2/itinerarium/api/infrastructure/webapp"
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
		transport.WithHandle("POST /inventory/move", transport.MoveInventoryItemHandler(services.Inventory)),
		transport.WithHandle("GET /search", transport.SearchDocumentsHandler(services.Documents)),
		transport.WithHandle("POST /import/obsidian", transport.ImportVaultHandler(services.VaultImport)),
	)

	opts := []transport.Option{
		transport.WithMiddleware(transport.Logging(logger)),
		transport.WithMiddleware(transport.SecurityHeaders(CSPFlag.Value(), HSTSFlag.Value())),
		transport.WithMiddleware(transport.MaxBytes(int64(BodyLimitFlag.Value()))),
		transport.WithHandle("GET /api/health", transport.HealthHandler()),
		transport.WithHandle("GET /api/setup", transport.SetupStatusHandler(services.Setup)),
		transport.WithHandle("POST /api/setup", transport.CreateInitialGMHandler(services.Setup)),
		transport.WithHandle("POST /api/login", transport.LoginHandler(services.Auth, loginThrottle, trustProxy)),
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
		transport.WithHandle("GET /users", transport.ListAccountsHandler(services.Users)),
		transport.WithHandle("POST /users", transport.CreateAccountHandler(services.Users)),
		transport.WithHandle(
			"POST /users/{id}/reset-password", transport.ResetPasswordHandler(services.Users, loginThrottle),
		),
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
		transport.WithHandle("GET /{id}/activity", transport.GetCharacterActivityHandler(services.Activity)),
		transport.WithSubRoute("/{id}/inventory", inventoryRouter(services, transport.CharacterOwner)),
		transport.WithSubRoute("/{id}/money", moneyRouter(services, transport.CharacterOwner)),
		transport.WithSubRoute("/{id}/journal", journalRouter(services)),
	)
}

// activityRouter serves the GM-wide campaign log and announcements under
// /api/activity. The per-character feed lives under
// /api/characters/{id}/activity.
func activityRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", transport.ListActivityHandler(services.Activity)),
		transport.WithHandle("POST /announcements", transport.AnnounceActivityHandler(services.Activity)),
	)
}

// journalRouter serves one character's journal entries under
// /api/characters/{id}/journal.
func journalRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", transport.ListJournalEntriesHandler(services.Journals)),
		transport.WithHandle("POST /", transport.CreateJournalEntryHandler(services.Journals)),
		transport.WithHandle("GET /{entryId}", transport.GetJournalEntryHandler(services.Journals)),
		transport.WithHandle("PATCH /{entryId}", transport.UpdateJournalEntryHandler(services.Journals)),
		transport.WithHandle("POST /{entryId}/convert", transport.ConvertJournalEntryHandler(services.Journals)),
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

// sessionsRouter serves sessions, participants, and game-day advances under
// /api/sessions.
func sessionsRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", transport.ListSessionsHandler(services.Sessions)),
		transport.WithHandle("POST /", transport.CreateSessionHandler(services.Sessions)),
		transport.WithHandle("GET /{id}", transport.GetSessionHandler(services.Sessions)),
		transport.WithHandle("PATCH /{id}", transport.UpdateSessionHandler(services.Sessions)),
		transport.WithHandle("POST /{id}/participants", transport.AddSessionParticipantHandler(services.Sessions)),
		transport.WithHandle(
			"DELETE /{id}/participants/{characterId}", transport.RemoveSessionParticipantHandler(services.Sessions),
		),
		transport.WithHandle("POST /{id}/game-day", transport.AdvanceSessionGameDayHandler(services.Sessions)),
	)
}

// currenciesRouter serves the currency catalog under /api/currencies.
func currenciesRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", transport.ListCurrenciesHandler(services.Catalog)),
		transport.WithHandle("POST /", transport.CreateCurrencyHandler(services.Catalog)),
		transport.WithHandle("POST /convert", transport.ConvertCurrencyHandler(services.Catalog)),
		transport.WithHandle("POST /simplify", transport.SimplifyCurrencyHandler(services.Catalog)),
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
		transport.WithHandle("GET /{id}/documents", transport.ListDocumentsHandler(services.Documents)),
		transport.WithHandle("POST /{id}/documents", transport.CreateDocumentHandler(services.Documents)),
		transport.WithHandle("GET /{id}/documents/tree", transport.GetDocumentFolderTreeHandler(services.Documents)),
	)
}

// documentsRouter serves single documents under /api/documents. Documents
// are created through their repository; reads and edits address the document
// directly.
func documentsRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /shared", transport.ListSharedDocumentsHandler(services.Documents)),
		transport.WithHandle("GET /{id}", transport.GetDocumentHandler(services.Documents)),
		transport.WithHandle("PATCH /{id}", transport.UpdateDocumentHandler(services.Documents)),
		transport.WithHandle("DELETE /{id}", transport.DeleteDocumentHandler(services.Documents)),
		transport.WithHandle("POST /{id}/share", transport.ShareDocumentHandler(services.Documents)),
		transport.WithHandle("GET /{id}/shares", transport.ListDocumentSharesHandler(services.Documents)),
		transport.WithHandle("POST /{id}/shares", transport.ShareDocumentWithCharacterHandler(services.Documents)),
		transport.WithHandle("DELETE /{id}/shares/{shareId}", transport.RevokeDocumentShareHandler(services.Documents)),
	)
}
