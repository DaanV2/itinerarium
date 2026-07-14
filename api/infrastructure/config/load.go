package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Load reads a YAML config file into the singleton. With an explicit path,
// a missing or invalid file is an error. With an empty path, config.yaml is
// searched for across [ConfigPaths]; not finding one is not an error (flags,
// env vars, and defaults still apply).
func Load(file string) error {
	if file != "" {
		v.SetConfigFile(file)

		return v.ReadInConfig()
	}

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	for _, p := range ConfigPaths() {
		v.AddConfigPath(p)
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			return nil
		}

		return err
	}

	return nil
}

// ConfigPaths returns, in search order, the directories checked for a
// config.yaml when Load is called with an empty path.
func ConfigPaths() []string {
	paths := []string{".config"}

	if dir, err := os.UserConfigDir(); err == nil {
		paths = append(paths, filepath.Join(dir, "itinerarium"))
	}

	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".itinerarium"))
	}

	return paths
}

// Save writes the fully resolved configuration (defaults, env vars, and
// flags already applied) as YAML to the first entry in [ConfigPaths].
func Save() error {
	return SaveAs(filepath.Join(ConfigPaths()[0], "config.yaml"))
}

// SaveAs writes the fully resolved configuration as YAML to an explicit path,
// creating its parent directory if necessary.
func SaveAs(file string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		return err
	}

	return v.WriteConfigAs(file)
}
