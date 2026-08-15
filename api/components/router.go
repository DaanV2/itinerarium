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

// CreateRouter assembles the HTTP router. Each resource is its own
// self-registering handler group: the group owns its full route paths (as
// constants in its file) and a Register method that wires them onto a router.
//
// Authenticated groups register onto an "authenticated" subrouter that applies
// RequireAuth once; it is mounted at the empty prefix so every group's full
// "/api/..." path is served unchanged, with RequireAuth wrapping each. Public
// groups (health, setup, login) register directly onto the root router.
// Security middleware (headers + body-size cap) comes from the security config
// set (M10).
func CreateRouter(services *Services, logger *log.Logger) *transport.Router {
	// One shared login/reset limiter for the whole server (nil when disabled by
	// config). Login and reset use distinct key prefixes, so sharing is safe.
	loginThrottle := transport.NewLoginThrottle(LoginMaxFailuresFlag.Value(), LoginLockoutFlag.Value())
	trustProxy := TrustProxyHeadersFlag.Value()

	authenticated := transport.NewRouter(transport.WithMiddleware(transport.RequireAuth(services.Auth)))
	registerAuthenticated(authenticated, services, loginThrottle)

	router := transport.NewRouter(
		transport.WithMiddleware(transport.Logging(logger)),
		transport.WithMiddleware(transport.SecurityHeaders(CSPFlag.Value(), HSTSFlag.Value())),
		transport.WithMiddleware(transport.MaxBytes(int64(BodyLimitFlag.Value()))),
		// Empty prefix: mount the authenticated group's full "/api/..." routes
		// unchanged, wrapped in RequireAuth.
		transport.WithSubRoute("", authenticated),
		transport.WithHandle("GET /api/health", transport.HealthHandler()),
	)

	// Public groups: reachable before (or without) authentication.
	setupv1.NewSetupHandler(services.Setup).Register(router)
	authenicationv1.NewAuthHandler(services.Auth, loginThrottle, trustProxy).Register(router)

	// Everything outside /api serves the frontend embedded in the binary.
	// Builds without the embedweb tag (dev, plain `go build`) are API-only;
	// there the vite dev server hosts the frontend instead.
	if assets, ok := webapp.Assets(); ok {
		router.Handle("/", transport.SPAHandler(assets))
	} else {
		logger.Warn("built without the embedded web UI (embedweb build tag), serving the API only")
	}

	return router
}

// registerAuthenticated wires every authenticated resource group onto r. Each
// group owns its own route paths and Register method; adding a resource means
// adding one line here.
func registerAuthenticated(r *transport.Router, services *Services, loginThrottle *transport.Throttle) {
	// Characters and their subresources.
	charactersv1.NewCharacterHandler(services.Characters).Register(r)
	locationsv1.NewCharacterLocationHandler(services.Locations).Register(r)
	activitiesv1.NewCharacterActivityHandler(services.Activity).Register(r)
	knowledgev1.NewCharacterJournalHandler(services.Journals).Register(r)
	inventoryv1.NewCharacterInventoryHandler(services.Inventory).Register(r)
	inventoryv1.NewCharacterMoneyHandler(services.Inventory).Register(r)

	// Groups and their subresources.
	sessionsv1.NewGroupHandler(services.Groups).Register(r)
	inventoryv1.NewGroupInventoryHandler(services.Inventory).Register(r)
	inventoryv1.NewGroupMoneyHandler(services.Inventory).Register(r)

	// Locations and their subresources.
	locationsv1.NewLocationHandler(services.Locations).Register(r)
	inventoryv1.NewLocationInventoryHandler(services.Inventory).Register(r)

	// Standalone resources.
	sessionsv1.NewSessionHandler(services.Sessions).Register(r)
	currenciesv1.NewCurrencyHandler(services.Catalog).Register(r)
	inventoryv1.NewItemHandler(services.Catalog).Register(r)
	inventoryv1.NewInventoryMoveHandler(services.Inventory).Register(r)
	knowledgev1.NewRepositoryHandler(services.Repositories, services.Documents).Register(r)
	knowledgev1.NewDocumentHandler(services.Documents).Register(r)
	activitiesv1.NewActivityHandler(services.Activity).Register(r)
	knowledgev1.NewSearchHandler(services.Documents).Register(r)
	knowledgev1.NewImportHandler(services.VaultImport).Register(r)

	// Account administration.
	usersv1.NewAdminUsersHandler(services.Users, loginThrottle).Register(r)
}
