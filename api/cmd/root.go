package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/DaanV2/itinerarium/api/infrastructure/config"
	"github.com/DaanV2/itinerarium/api/infrastructure/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

		bindDatabaseFlags(cmd)
		logging.Setup()

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().String(
		"config", "", "path to a YAML config file (auto-discovered from standard locations if omitted)",
	)

	logFlags := pflag.NewFlagSet("log", pflag.ContinueOnError)
	logFlags.String("level", "info", "log level: debug, info, warn, error, fatal")
	logFlags.String("format", "text", "log format: text, json, logfmt")
	logFlags.Bool("report-caller", false, "include the file:line that emitted each log entry")
	rootCmd.PersistentFlags().AddFlagSet(logFlags)
	config.MustBindFlags("log", logFlags)

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
