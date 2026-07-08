package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/config"
)

func TestDefaultsApply(t *testing.T) {
	cfg := config.GetContext("test-defaults")
	if got := cfg.String("some-key", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}

	if got := cfg.Int("some-int", 42); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
}

func TestEnvOverridesDefault(t *testing.T) {
	t.Setenv("TESTENV_DATABASE_PATH", "/tmp/override.db")

	cfg := config.GetContext("testenv")
	if got := cfg.String("database-path", "data/default.db"); got != "/tmp/override.db" {
		t.Fatalf("expected env override, got %q", got)
	}
}

func TestContextsAreCached(t *testing.T) {
	first := config.GetContext("cached")

	second := config.GetContext("cached")
	if first != second {
		t.Fatal("expected the same Context instance for the same component")
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

func TestSaveAsWritesResolvedConfig(t *testing.T) {
	cfg := config.GetContext("save-test")
	cfg.String("some-key", "some-value")

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
