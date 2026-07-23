package cmd

import (
	"context"
	"time"

	"github.com/DaanV2/itinerarium/api/components"
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/transport/server"
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
	fs := serveCmd.Flags()
	server.ServerConfigSet.AddToSet(fs)
	persistence.DatabaseConfigSet.AddToSet(fs)
	authentication.AuthConfigSet.AddToSet(fs)
	components.CatalogConfigSet.AddToSet(fs)
	components.SecurityConfigSet.AddToSet(fs)
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
