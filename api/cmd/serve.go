package cmd

import (
	"context"
	"time"

	"github.com/DaanV2/itinerarium/api/components"
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/config"
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
	serveCmd.Flags().String("keys-path", "data/keys", "directory holding the RS512 JWT signing key pair")
	serveCmd.Flags().Duration("token-ttl", authentication.DefaultTokenTTL, "access token lifetime")
	serveCmd.Flags().String("catalog-path", "", "optional JSON/YAML file seeding the currency and item catalog on startup")
	config.MustBindFlags("server", serveCmd.Flags())
	addDatabaseFlags(serveCmd)
}

func runServe(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	logger := log.Default()

	server, err := components.BuildServer(ctx)
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() { errCh <- server.Server.ListenAndServe() }()
	logger.Info("server started", "address", server.Server.Addr())

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down", "timeout", shutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}
