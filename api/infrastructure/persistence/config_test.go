package persistence_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
)

func TestOptionsRejectsUnknownType(t *testing.T) {
	_, err := persistence.DatabaseConfig{Type: "oracle"}.Options()
	if err == nil {
		t.Fatal("expected an error for an unsupported database type")
	}
}

func TestOptionsPostgresRequiresDSN(t *testing.T) {
	_, err := persistence.DatabaseConfig{Type: "postgres"}.Options()
	if err == nil {
		t.Fatal("expected postgres without a DSN to error")
	}
}

func TestOptionsMySQLRequiresDSN(t *testing.T) {
	_, err := persistence.DatabaseConfig{Type: "mysql"}.Options()
	if err == nil {
		t.Fatal("expected mysql without a DSN to error")
	}
}

func TestOptionsSQLiteDefaults(t *testing.T) {
	// SQLite falls back to the file path and needs no DSN.
	opts, err := persistence.DatabaseConfig{
		Type: "sqlite",
		Path: "data/itinerarium.db",
	}.Options()
	if err != nil {
		t.Fatalf("Options: %v", err)
	}

	if len(opts) == 0 {
		t.Fatal("expected sqlite options to be produced")
	}
}

func TestOptionsMemory(t *testing.T) {
	opts, err := persistence.DatabaseConfig{Type: "memory"}.Options()
	if err != nil {
		t.Fatalf("Options: %v", err)
	}

	if len(opts) == 0 {
		t.Fatal("expected memory options to be produced")
	}
}

func TestGetConfigReadsFlagDefaults(t *testing.T) {
	cfg := persistence.GetConfig()

	if cfg.Type != persistence.SQLite.String() {
		t.Fatalf("expected default type sqlite, got %q", cfg.Type)
	}

	if cfg.Path != "data/itinerarium.db" {
		t.Fatalf("expected default sqlite path, got %q", cfg.Path)
	}
}

func TestDatabaseConfigSetValidatesType(t *testing.T) {
	t.Setenv("DATABASE_TYPE", "oracle")

	if err := persistence.DatabaseConfigSet.Validate(); err == nil {
		t.Fatal("expected validation to reject an unknown database type")
	}
}
