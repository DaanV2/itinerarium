package components

import (
	"context"

	"github.com/DaanV2/itinerarium/api/infrastructure/lifecycle"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/servers"
	"github.com/charmbracelet/log"
)

// ServerComponents holds the assembled server and the pieces callers need to
// run it, inspect it, or extend it. Shutdown drives the graceful-shutdown
// lifecycle over everything that holds resources.
type ServerComponents struct {
	Config       *ServerConfig
	DB           *persistence.Database
	Repositories *Repositories
	Services     *Services
	Server       *servers.Server
}

// BuildServer wires every component into a ready-to-run server: database and
// migrations, authentication, repositories, application services, the router,
// and the HTTP server. It optionally seeds the currency/item catalog when a
// catalog file is configured.
func BuildServer(ctx context.Context) (*ServerComponents, error) {
	cfg, err := LoadServerConfig()
	if err != nil {
		return nil, err
	}

	logger := log.Default()

	db, err := SetupDatabase(cfg)
	if err != nil {
		return nil, err
	}

	repos := NewRepositories(db)

	tokens, err := SetupAuthentication(cfg, repos.RevokedTokens)
	if err != nil {
		return nil, err
	}

	services := NewServices(repos, tokens)

	if err := seedCatalog(ctx, cfg, services, logger); err != nil {
		return nil, err
	}

	server := servers.New(
		servers.WithAddr(cfg.Address.Listen()),
		servers.WithHandler(CreateRouter(services, logger)),
	)

	return &ServerComponents{
		Config:       cfg,
		DB:           db,
		Repositories: repos,
		Services:     services,
		Server:       server,
	}, nil
}

// Shutdown drives the graceful-shutdown lifecycle: the HTTP server drains
// first, then the database connection closes.
func (c *ServerComponents) Shutdown(ctx context.Context) error {
	return lifecycle.ShutdownAll(ctx, c.Server, c.DB)
}

// seedCatalog loads the configured currency/item catalog file, if any.
func seedCatalog(ctx context.Context, cfg *ServerConfig, services *Services, logger *log.Logger) error {
	if cfg.CatalogPath == "" {
		return nil
	}

	curCount, itemCount, err := services.Catalog.LoadFile(ctx, cfg.CatalogPath)
	if err != nil {
		return err
	}

	logger.Info("catalog seeded", "path", cfg.CatalogPath, "currencies", curCount, "items", itemCount)

	return nil
}
