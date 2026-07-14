package cmd

import (
	"context"

	"github.com/DaanV2/itinerarium/api/components"
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
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
	persistence.DatabaseConfigSet.AddToSet(initCmd.Flags())
	authentication.AuthConfigSet.AddToSet(initCmd.Flags())
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	logger := log.Default()

	email, err := cmd.Flags().GetString("email")
	if err != nil {
		return err
	}

	password, err := cmd.Flags().GetString("password")
	if err != nil {
		return err
	}

	db, err := components.SetupDatabase()
	if err != nil {
		return err
	}
	defer func() { _ = db.Shutdown(context.Background()) }()

	repos := components.NewRepositories(db)

	tokens, err := components.SetupAuthentication(repos.RevokedTokens)
	if err != nil {
		return err
	}

	services := components.NewServices(repos, tokens)

	user, _, err := services.Setup.CreateInitialGM(ctx, email, password)
	if err != nil {
		return err
	}

	logger.Info("created initial GM account", "email", user.Email, "id", user.ID)

	return nil
}
