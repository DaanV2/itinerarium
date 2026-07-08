package cmd

import (
	"fmt"

	"github.com/DaanV2/itinerarium/api/infrastructure/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect or persist the resolved configuration",
}

var configPathsCmd = &cobra.Command{
	Use:   "paths",
	Short: "List the directories searched for a config.yaml",
	RunE:  runConfigPaths,
}

var configSaveCmd = &cobra.Command{
	Use:   "save",
	Short: "Write the resolved configuration to a YAML file",
	RunE:  runConfigSave,
}

func init() {
	configSaveCmd.Flags().StringP("path", "p", "", "path to write the config file to (defaults to the first search path)")
	configCmd.AddCommand(configPathsCmd, configSaveCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigPaths(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()
	for _, p := range config.ConfigPaths() {
		if _, err := fmt.Fprintln(out, p); err != nil {
			return err
		}
	}

	return nil
}

func runConfigSave(cmd *cobra.Command, _ []string) error {
	path, err := cmd.Flags().GetString("path")
	if err != nil {
		return err
	}

	if path == "" {
		return config.Save()
	}

	return config.SaveAs(path)
}
