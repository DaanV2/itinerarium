package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/DaanV2/itinerarium/api/infrastructure/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "itinerarium",
	Short:         "Itinerarium — self-hosted TTRPG campaign tool API",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		file, err := cmd.Root().PersistentFlags().GetString("config")
		if err != nil {
			return err
		}

		if err := config.Load(file); err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().String("config", "", "path to a YAML config file")
	rootCmd.AddCommand(serveCmd)
}

// Execute runs the CLI. It cancels the command context on SIGINT/SIGTERM so
// commands can coordinate a graceful shutdown (see cmd/serve.go).
func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := rootCmd.ExecuteContext(ctx)
	stop()

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
