package config_test

import (
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
