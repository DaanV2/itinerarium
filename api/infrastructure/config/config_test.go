package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/config"
	"github.com/spf13/pflag"
)

func TestFlagDefaultApplies(t *testing.T) {
	set := config.New("test-defaults")
	strFlag := set.String("test-defaults.some-key", "fallback", "test flag")
	intFlag := set.Int("test-defaults.some-int", 42, "test flag")

	if got := strFlag.Value(); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}

	if got := intFlag.Value(); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
}

func TestEnvOverridesDefault(t *testing.T) {
	set := config.New("test-env")
	f := set.String("testenv.database-path", "data/default.db", "test flag")

	t.Setenv("TESTENV_DATABASE_PATH", "/tmp/override.db")

	if got := f.Value(); got != "/tmp/override.db" {
		t.Fatalf("expected env override, got %q", got)
	}
}

func TestFlagOverridesEnv(t *testing.T) {
	set := config.New("test-flag-priority")
	f := set.String("testprio.value", "default", "test flag")

	t.Setenv("TESTPRIO_VALUE", "from-env")

	fs := pflag.NewFlagSet("cmd", pflag.ContinueOnError)
	set.AddToSet(fs)

	if err := fs.Set("testprio.value", "from-flag"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	if got := f.Value(); got != "from-flag" {
		t.Fatalf("expected the flag to win over env, got %q", got)
	}
}

func TestUsageAdvertisesEnvVar(t *testing.T) {
	set := config.New("test-usage")
	set.String("testusage.some-key", "", "does things")

	fs := pflag.NewFlagSet("cmd", pflag.ContinueOnError)
	set.AddToSet(fs)

	f := fs.Lookup("testusage.some-key")
	if f == nil {
		t.Fatal("expected the flag to be registered on the command set")
	}

	if !strings.Contains(f.Usage, "TESTUSAGE_SOME_KEY") {
		t.Fatalf("expected usage to advertise the env var, got %q", f.Usage)
	}
}

func TestValidateRunsEverySet(t *testing.T) {
	sentinel := errors.New("always invalid")
	config.New("test-validate").WithValidate(func(*config.Config) error { return sentinel })

	if err := config.Validate(); !errors.Is(err, sentinel) {
		t.Fatalf("expected Validate to surface the set's error, got %v", err)
	}
}

func TestGetReturnsRegisteredSet(t *testing.T) {
	created := config.New("test-get")
	if got := config.Get("test-get"); got != created {
		t.Fatal("expected Get to return the registered set instance")
	}
}

func TestEnvName(t *testing.T) {
	if got := config.EnvName("database.max-idle-conns"); got != "DATABASE_MAX_IDLE_CONNS" {
		t.Fatalf("expected DATABASE_MAX_IDLE_CONNS, got %q", got)
	}
}

func TestConfigPathsStartsWithDotConfig(t *testing.T) {
	paths := config.ConfigPaths()
	if len(paths) == 0 || paths[0] != ".config" {
		t.Fatalf("expected first search path to be \".config\", got %v", paths)
	}
}

func TestLoadMissingExplicitFileErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	if err := config.Load(file); err == nil {
		t.Fatal("expected an error for a missing explicit config file")
	}
}

func TestLoadWithoutFileToleratesNoConfig(t *testing.T) {
	if err := config.Load(""); err != nil {
		t.Fatalf("expected auto-discovery to tolerate a missing config.yaml, got %v", err)
	}
}

func TestLoadReadsYAMLValues(t *testing.T) {
	set := config.New("test-yaml")
	f := set.String("testyaml.value", "default", "test flag")

	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(file, []byte("testyaml:\n  value: from-yaml\n"), 0o600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	if err := config.Load(file); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := f.Value(); got != "from-yaml" {
		t.Fatalf("expected the YAML value, got %q", got)
	}
}

func TestSaveAsWritesResolvedConfig(t *testing.T) {
	set := config.New("test-save")
	set.String("testsave.some-key", "some-value", "test flag")

	file := filepath.Join(t.TempDir(), "nested", "config.yaml")
	if err := config.SaveAs(file); err != nil {
		t.Fatalf("SaveAs failed: %v", err)
	}

	data, err := os.ReadFile(file) //nolint:gosec // file is a t.TempDir() path this test just wrote itself
	if err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}

	if !strings.Contains(string(data), "some-value") {
		t.Fatalf("expected saved config to contain the resolved value, got: %s", data)
	}
}
