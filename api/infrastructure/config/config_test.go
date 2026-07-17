package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/config"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestFlagDefaultApplies(t *testing.T) {
	set := config.New("test-defaults")
	strFlag := set.String("test-defaults.some-key", "fallback", "test flag")
	intFlag := set.Int("test-defaults.some-int", 42, "test flag")

	require.Equal(t, "fallback", strFlag.Value())
	require.Equal(t, 42, intFlag.Value())
}

func TestEnvOverridesDefault(t *testing.T) {
	set := config.New("test-env")
	f := set.String("testenv.database-path", "data/default.db", "test flag")

	t.Setenv("TESTENV_DATABASE_PATH", "/tmp/override.db")

	require.Equal(t, "/tmp/override.db", f.Value(), "env should override the default")
}

func TestFlagOverridesEnv(t *testing.T) {
	set := config.New("test-flag-priority")
	f := set.String("testprio.value", "default", "test flag")

	t.Setenv("TESTPRIO_VALUE", "from-env")

	fs := pflag.NewFlagSet("cmd", pflag.ContinueOnError)
	set.AddToSet(fs)

	require.NoError(t, fs.Set("testprio.value", "from-flag"), "setting flag")

	require.Equal(t, "from-flag", f.Value(), "the flag should win over env")
}

func TestUsageAdvertisesEnvVar(t *testing.T) {
	set := config.New("test-usage")
	set.String("testusage.some-key", "", "does things")

	fs := pflag.NewFlagSet("cmd", pflag.ContinueOnError)
	set.AddToSet(fs)

	f := fs.Lookup("testusage.some-key")
	require.NotNil(t, f, "the flag should be registered on the command set")

	require.Contains(t, f.Usage, "TESTUSAGE_SOME_KEY", "usage should advertise the env var")
}

func TestValidateRunsEverySet(t *testing.T) {
	sentinel := errors.New("always invalid")
	config.New("test-validate").WithValidate(func(*config.Config) error { return sentinel })

	require.ErrorIs(t, config.Validate(), sentinel, "Validate should surface the set's error")
}

func TestGetReturnsRegisteredSet(t *testing.T) {
	created := config.New("test-get")
	require.Same(t, created, config.Get("test-get"), "Get should return the registered set instance")
}

func TestEnvName(t *testing.T) {
	require.Equal(t, "DATABASE_MAX_IDLE_CONNS", config.EnvName("database.max-idle-conns"))
}

func TestConfigPathsStartsWithDotConfig(t *testing.T) {
	paths := config.ConfigPaths()
	require.NotEmpty(t, paths)
	require.Equal(t, ".config", paths[0], "first search path should be .config")
}

func TestLoadMissingExplicitFileErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	require.Error(t, config.Load(file), "a missing explicit config file should error")
}

func TestLoadWithoutFileToleratesNoConfig(t *testing.T) {
	require.NoError(t, config.Load(""), "auto-discovery should tolerate a missing config.yaml")
}

func TestLoadReadsYAMLValues(t *testing.T) {
	set := config.New("test-yaml")
	f := set.String("testyaml.value", "default", "test flag")

	file := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(file, []byte("testyaml:\n  value: from-yaml\n"), 0o600), "writing config file")

	require.NoError(t, config.Load(file), "Load")

	require.Equal(t, "from-yaml", f.Value(), "expected the YAML value")
}

func TestSaveAsWritesResolvedConfig(t *testing.T) {
	set := config.New("test-save")
	set.String("testsave.some-key", "some-value", "test flag")

	file := filepath.Join(t.TempDir(), "nested", "config.yaml")
	require.NoError(t, config.SaveAs(file), "SaveAs")

	data, err := os.ReadFile(file) //nolint:gosec // file is a t.TempDir() path this test just wrote itself
	require.NoError(t, err, "expected config file to exist")

	require.Contains(t, string(data), "some-value", "saved config should contain the resolved value")
}
