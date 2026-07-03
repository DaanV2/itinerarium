package cmd

import (
	"context"
	"time"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/config"
	"github.com/DaanV2/itinerarium/api/infrastructure/lifecycle"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/DaanV2/itinerarium/api/infrastructure/servers"
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

// shutdownTimeout bounds the graceful-shutdown phase after the context is
// cancelled (mechanus convention: 1 minute).
const shutdownTimeout = time.Minute

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the API server",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().String("address", ":8080", "address the API server listens on")
	serveCmd.Flags().String("database-path", "data/itinerarium.db", "path to the SQLite database file")
	serveCmd.Flags().String("keys-path", "data/keys", "directory holding the RS512 JWT signing key pair")
	serveCmd.Flags().Duration("token-ttl", authentication.DefaultTokenTTL, "access token lifetime")
	config.MustBindFlags("server", serveCmd.Flags())
}

func runServe(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	cfg := config.GetContext("server")
	logger := log.Default()

	db, err := persistence.New(
		persistence.WithPath(cfg.String("database-path", "data/itinerarium.db")),
	)
	if err != nil {
		return err
	}
	if err := db.Migrate(); err != nil {
		return err
	}

	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(cfg.String("keys-path", "data/keys")))
	if err != nil {
		return err
	}

	tokens := authentication.NewTokenService(
		keys,
		repositories.NewRevokedTokens(db),
		authentication.WithTTL(cfg.Duration("token-ttl", authentication.DefaultTokenTTL)),
	)
	users := repositories.NewUsers(db)
	setupSvc := application.NewSetupService(users, tokens)
	authSvc := application.NewAuthService(tokens, users)
	userSvc := application.NewUserService(users)
	requireAuth := transport.RequireAuth(authSvc)

	router := transport.NewRouter(
		transport.WithMiddleware(transport.Logging(logger)),
		transport.WithHandle("GET /api/health", transport.HealthHandler()),
		transport.WithHandle("GET /api/setup", transport.SetupStatusHandler(setupSvc)),
		transport.WithHandle("POST /api/setup", transport.CreateInitialGMHandler(setupSvc)),
		transport.WithHandle("GET /api/admin/users", requireAuth(transport.ListAccountsHandler(userSvc))),
		transport.WithHandle("POST /api/admin/users", requireAuth(transport.CreateAccountHandler(userSvc))),
		transport.WithHandle(
			"POST /api/admin/users/{id}/reset-password",
			requireAuth(transport.ResetPasswordHandler(userSvc)),
		),
	)
	server := servers.New(
		servers.WithAddr(cfg.String("address", ":8080")),
		servers.WithHandler(router),
	)

	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()
	logger.Info("server started", "address", server.Addr())

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down", "timeout", shutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	return lifecycle.ShutdownAll(shutdownCtx, server, db)
}
