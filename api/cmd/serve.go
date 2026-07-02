package cmd

import (
	"context"
	"time"

	"github.com/DaanV2/itinerarium/api/infrastructure/config"
	"github.com/DaanV2/itinerarium/api/infrastructure/lifecycle"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
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

	router := transport.NewRouter(
		transport.WithMiddleware(transport.Logging(logger)),
		transport.WithHandle("GET /api/health", transport.HealthHandler()),
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
