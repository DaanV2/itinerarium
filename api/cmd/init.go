package cmd

import (
	"context"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/config"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create the installation's initial GM account",
	Long: "Bootstraps a fresh installation with its first GM account from the command line, " +
		"without going through the web setup wizard — useful for headless deployments. " +
		"Fails once any account already exists.",
	RunE: runInit,
}

func init() {
	initCmd.Flags().String("email", "", "email address for the initial GM account")
	initCmd.Flags().String("password", "", "password for the initial GM account")
	_ = initCmd.MarkFlagRequired("email")
	_ = initCmd.MarkFlagRequired("password")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	cfg := config.GetContext("server")
	logger := log.Default()

	email, err := cmd.Flags().GetString("email")
	if err != nil {
		return err
	}

	password, err := cmd.Flags().GetString("password")
	if err != nil {
		return err
	}

	db, err := persistence.New(
		persistence.WithPath(cfg.String("database-path", "data/itinerarium.db")),
	)
	if err != nil {
		return err
	}
	defer func() { _ = db.Shutdown(context.Background()) }()

	if err := db.Migrate(); err != nil {
		return err
	}

	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(cfg.String("keys-path", "data/keys")))
	if err != nil {
		return err
	}

	tokens := authentication.NewTokenService(keys, repositories.NewRevokedTokens(db))
	setupSvc := application.NewSetupService(repositories.NewUsers(db), tokens)

	user, _, err := setupSvc.CreateInitialGM(ctx, email, password)
	if err != nil {
		return err
	}

	logger.Info("created initial GM account", "email", user.Email, "id", user.ID)

	return nil
}
