package components

import (
	activitiesv1 "github.com/DaanV2/itinerarium/api/api/v1/activities"
	authenicationv1 "github.com/DaanV2/itinerarium/api/api/v1/authenication"
	charactersv1 "github.com/DaanV2/itinerarium/api/api/v1/characters"
	currenciesv1 "github.com/DaanV2/itinerarium/api/api/v1/currencies"
	inventoryv1 "github.com/DaanV2/itinerarium/api/api/v1/inventory"
	knowledgev1 "github.com/DaanV2/itinerarium/api/api/v1/knowledge"
	locationsv1 "github.com/DaanV2/itinerarium/api/api/v1/locations"
	sessionsv1 "github.com/DaanV2/itinerarium/api/api/v1/sessions"
	setupv1 "github.com/DaanV2/itinerarium/api/api/v1/setup"
	usersv1 "github.com/DaanV2/itinerarium/api/api/v1/users"
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
		transport.WithHandle("POST /inventory/move", inventoryv1.MoveInventoryItemHandler(services.Inventory)),
		transport.WithHandle("GET /search", knowledgev1.SearchDocumentsHandler(services.Documents)),
		transport.WithHandle("POST /import/obsidian", knowledgev1.ImportVaultHandler(services.VaultImport)),
	)

	opts := []transport.Option{
		transport.WithMiddleware(transport.Logging(logger)),
		transport.WithMiddleware(transport.SecurityHeaders(CSPFlag.Value(), HSTSFlag.Value())),
		transport.WithMiddleware(transport.MaxBytes(int64(BodyLimitFlag.Value()))),
		transport.WithHandle("GET /api/health", transport.HealthHandler()),
		transport.WithHandle("GET /api/setup", setupv1.SetupStatusHandler(services.Setup)),
		transport.WithHandle("POST /api/setup", setupv1.CreateInitialGMHandler(services.Setup)),
		transport.WithHandle("POST /api/login", authenicationv1.LoginHandler(services.Auth, loginThrottle, trustProxy)),
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
		transport.WithHandle("GET /users", usersv1.ListAccountsHandler(services.Users)),
		transport.WithHandle("POST /users", usersv1.CreateAccountHandler(services.Users)),
		transport.WithHandle(
			"POST /users/{id}/reset-password", usersv1.ResetPasswordHandler(services.Users, loginThrottle),
		),
	)
}

// charactersRouter serves characters and their nested inventory/money
// subresources under /api/characters.
func charactersRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", charactersv1.ListCharactersHandler(services.Characters)),
		transport.WithHandle("POST /", charactersv1.CreateCharacterHandler(services.Characters)),
		transport.WithHandle("GET /{id}", charactersv1.GetCharacterHandler(services.Characters)),
		transport.WithHandle("PATCH /{id}", charactersv1.UpdateCharacterHandler(services.Characters)),
		transport.WithHandle("PUT /{id}/location", locationsv1.SetCharacterLocationHandler(services.Locations)),
		transport.WithHandle("DELETE /{id}/location", locationsv1.ClearCharacterLocationHandler(services.Locations)),
		transport.WithHandle("GET /{id}/activity", activitiesv1.GetCharacterActivityHandler(services.Activity)),
		transport.WithSubRoute("/{id}/inventory", inventoryRouter(services, inventoryv1.CharacterOwner)),
		transport.WithSubRoute("/{id}/money", moneyRouter(services, inventoryv1.CharacterOwner)), // TODO think this handler is in the wrong place, or inventoryv1.CharacterOwner needs to get shared
		transport.WithSubRoute("/{id}/journal", journalRouter(services)),
	)
}

// activityRouter serves the GM-wide campaign log and announcements under
// /api/activity. The per-character feed lives under
// /api/characters/{id}/activity.
func activityRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", activitiesv1.ListActivityHandler(services.Activity)),
		transport.WithHandle("POST /announcements", activitiesv1.AnnounceActivityHandler(services.Activity)),
	)
}

// journalRouter serves one character's journal entries under
// /api/characters/{id}/journal.
func journalRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", knowledgev1.ListJournalEntriesHandler(services.Journals)),
		transport.WithHandle("POST /", knowledgev1.CreateJournalEntryHandler(services.Journals)),
		transport.WithHandle("GET /{entryId}", knowledgev1.GetJournalEntryHandler(services.Journals)),
		transport.WithHandle("PATCH /{entryId}", knowledgev1.UpdateJournalEntryHandler(services.Journals)),
		transport.WithHandle("POST /{entryId}/convert", knowledgev1.ConvertJournalEntryHandler(services.Journals)),
	)
}

// locationsRouter serves locations and their access grants under
// /api/locations.
func locationsRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", locationsv1.ListLocationsHandler(services.Locations)),
		transport.WithHandle("POST /", locationsv1.CreateLocationHandler(services.Locations)),
		transport.WithHandle("GET /{id}", locationsv1.GetLocationHandler(services.Locations)),
		transport.WithHandle("PATCH /{id}", locationsv1.UpdateLocationHandler(services.Locations)),
		transport.WithHandle("GET /{id}/access", locationsv1.ListLocationAccessHandler(services.Locations)),
		transport.WithHandle("POST /{id}/access", locationsv1.GrantLocationAccessHandler(services.Locations)),
		transport.WithHandle("DELETE /{id}/access/{accessId}", locationsv1.RevokeLocationAccessHandler(services.Locations)),
		transport.WithSubRoute("/{id}/inventory", inventoryRouter(services, inventoryv1.LocationOwner)),
	)
}

// inventoryRouter serves one inventory (character, group, or location — the
// extractor decides what {id} names) under <resource>/{id}/inventory.
func inventoryRouter(services *Services, owner inventoryv1.OwnerExtractor) *transport.Router {
	// TODO: these routers get reused but probably shouldn't, handlers for specific owners should just be explicit
	return transport.NewRouter(
		transport.WithHandle("GET /", inventoryv1.ListInventoryHandler(services.Inventory, owner)),
		transport.WithHandle("POST /", inventoryv1.AddInventoryItemHandler(services.Inventory, owner)),
		transport.WithHandle("PATCH /{itemId}", inventoryv1.UpdateInventoryItemHandler(services.Inventory, owner)),
		transport.WithHandle("DELETE /{itemId}", inventoryv1.RemoveInventoryItemHandler(services.Inventory, owner)),
	)
}

// moneyRouter serves one owner's balances (character or group) under
// <resource>/{id}/money.
func moneyRouter(services *Services, owner inventoryv1.OwnerExtractor) *transport.Router {
	return transport.NewRouter(
		// TODO is inventoryv1 correct place for ListMoneyHandler and SetMoneyHandler, should this then be moved to inventory router?
		transport.WithHandle("GET /", inventoryv1.ListMoneyHandler(services.Inventory, owner)),
		transport.WithHandle("PUT /{currencyId}", inventoryv1.SetMoneyHandler(services.Inventory, owner)),
	)
}

// groupsRouter serves groups and membership under /api/groups.
func groupsRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", sessionsv1.ListGroupsHandler(services.Groups)),
		transport.WithHandle("POST /", sessionsv1.CreateGroupHandler(services.Groups)),
		transport.WithHandle("GET /{id}", sessionsv1.GetGroupHandler(services.Groups)),
		transport.WithHandle("PATCH /{id}", sessionsv1.UpdateGroupHandler(services.Groups)),
		transport.WithHandle("POST /{id}/members", sessionsv1.JoinGroupHandler(services.Groups)),
		transport.WithHandle("DELETE /{id}/members/{characterId}", sessionsv1.LeaveGroupHandler(services.Groups)),
		transport.WithSubRoute("/{id}/inventory", inventoryRouter(services, inventoryv1.GroupOwner)),
		transport.WithSubRoute("/{id}/money", moneyRouter(services, inventoryv1.GroupOwner)),
	)
}

// sessionsRouter serves sessions, participants, and game-day advances under
// /api/sessions.
func sessionsRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", sessionsv1.ListSessionsHandler(services.Sessions)),
		transport.WithHandle("POST /", sessionsv1.CreateSessionHandler(services.Sessions)),
		transport.WithHandle("GET /{id}", sessionsv1.GetSessionHandler(services.Sessions)),
		transport.WithHandle("PATCH /{id}", sessionsv1.UpdateSessionHandler(services.Sessions)),
		transport.WithHandle("POST /{id}/participants", sessionsv1.AddSessionParticipantHandler(services.Sessions)),
		transport.WithHandle(
			"DELETE /{id}/participants/{characterId}", sessionsv1.RemoveSessionParticipantHandler(services.Sessions),
		),
		transport.WithHandle("POST /{id}/game-day", sessionsv1.AdvanceSessionGameDayHandler(services.Sessions)),
	)
}

// currenciesRouter serves the currency catalog under /api/currencies.
func currenciesRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", currenciesv1.ListCurrenciesHandler(services.Catalog)),
		transport.WithHandle("POST /", currenciesv1.CreateCurrencyHandler(services.Catalog)),
		transport.WithHandle("POST /convert", currenciesv1.ConvertCurrencyHandler(services.Catalog)),
		transport.WithHandle("POST /simplify", currenciesv1.SimplifyCurrencyHandler(services.Catalog)),
	)
}

// itemsRouter serves the item-definition catalog under /api/items.
func itemsRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", inventoryv1.ListItemDefinitionsHandler(services.Catalog)),
		transport.WithHandle("POST /", inventoryv1.CreateItemDefinitionHandler(services.Catalog)),
	)
}

// repositoriesRouter serves knowledge repositories (read-only — they are
// provisioned automatically, never created by a caller) under
// /api/repositories.
func repositoriesRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /", knowledgev1.ListRepositoriesHandler(services.Repositories)),
		transport.WithHandle("GET /{id}", knowledgev1.GetRepositoryHandler(services.Repositories)),
		transport.WithHandle("GET /{id}/documents", knowledgev1.ListDocumentsHandler(services.Documents)),
		transport.WithHandle("POST /{id}/documents", knowledgev1.CreateDocumentHandler(services.Documents)),
		transport.WithHandle("GET /{id}/documents/tree", knowledgev1.GetDocumentFolderTreeHandler(services.Documents)),
	)
}

// documentsRouter serves single documents under /api/documents. Documents
// are created through their repository; reads and edits address the document
// directly.
func documentsRouter(services *Services) *transport.Router {
	return transport.NewRouter(
		transport.WithHandle("GET /shared", knowledgev1.ListSharedDocumentsHandler(services.Documents)),
		transport.WithHandle("GET /{id}", knowledgev1.GetDocumentHandler(services.Documents)),
		transport.WithHandle("PATCH /{id}", knowledgev1.UpdateDocumentHandler(services.Documents)),
		transport.WithHandle("DELETE /{id}", knowledgev1.DeleteDocumentHandler(services.Documents)),
		transport.WithHandle("POST /{id}/share", knowledgev1.ShareDocumentHandler(services.Documents)),
		transport.WithHandle("GET /{id}/shares", knowledgev1.ListDocumentSharesHandler(services.Documents)),
		transport.WithHandle("POST /{id}/shares", knowledgev1.ShareDocumentWithCharacterHandler(services.Documents)),
		transport.WithHandle("DELETE /{id}/shares/{shareId}", knowledgev1.RevokeDocumentShareHandler(services.Documents)),
	)
}
