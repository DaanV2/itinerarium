package components_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/components"
)

func TestDatabaseOptionsRejectsUnknownType(t *testing.T) {
	_, err := components.DatabaseOptions(components.DatabaseConfig{Type: "oracle"})
	if err == nil {
		t.Fatal("expected an error for an unsupported database type")
	}
}

func TestDatabaseOptionsPostgresRequiresDSN(t *testing.T) {
	_, err := components.DatabaseOptions(components.DatabaseConfig{Type: "postgres"})
	if err == nil {
		t.Fatal("expected postgres without a DSN to error")
	}
}

func TestDatabaseOptionsMySQLRequiresDSN(t *testing.T) {
	_, err := components.DatabaseOptions(components.DatabaseConfig{Type: "mysql"})
	if err == nil {
		t.Fatal("expected mysql without a DSN to error")
	}
}

func TestDatabaseOptionsSQLiteDefaults(t *testing.T) {
	// SQLite falls back to the file path and needs no DSN.
	opts, err := components.DatabaseOptions(components.DatabaseConfig{
		Type: "sqlite",
		Path: "data/itinerarium.db",
	})
	if err != nil {
		t.Fatalf("DatabaseOptions: %v", err)
	}

	if len(opts) == 0 {
		t.Fatal("expected sqlite options to be produced")
	}
}

func TestDatabaseOptionsMemory(t *testing.T) {
	opts, err := components.DatabaseOptions(components.DatabaseConfig{Type: "memory"})
	if err != nil {
		t.Fatalf("DatabaseOptions: %v", err)
	}

	if len(opts) == 0 {
		t.Fatal("expected memory options to be produced")
	}
}
