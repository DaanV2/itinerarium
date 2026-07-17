package persistence_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/stretchr/testify/require"
)

func TestOptionsRejectsUnknownType(t *testing.T) {
	_, err := persistence.DatabaseConfig{Type: "oracle"}.Options()
	require.Error(t, err, "an unsupported database type should error")
}

func TestOptionsPostgresRequiresDSN(t *testing.T) {
	_, err := persistence.DatabaseConfig{Type: "postgres"}.Options()
	require.Error(t, err, "postgres without a DSN should error")
}

func TestOptionsMySQLRequiresDSN(t *testing.T) {
	_, err := persistence.DatabaseConfig{Type: "mysql"}.Options()
	require.Error(t, err, "mysql without a DSN should error")
}

func TestOptionsSQLiteDefaults(t *testing.T) {
	// SQLite falls back to the file path and needs no DSN.
	opts, err := persistence.DatabaseConfig{
		Type: "sqlite",
		Path: "data/itinerarium.db",
	}.Options()
	require.NoError(t, err, "Options")

	require.NotEmpty(t, opts, "expected sqlite options to be produced")
}

func TestOptionsMemory(t *testing.T) {
	opts, err := persistence.DatabaseConfig{Type: "memory"}.Options()
	require.NoError(t, err, "Options")

	require.NotEmpty(t, opts, "expected memory options to be produced")
}

func TestGetConfigReadsFlagDefaults(t *testing.T) {
	cfg := persistence.GetConfig()

	require.Equal(t, persistence.SQLite.String(), cfg.Type)
	require.Equal(t, "data/itinerarium.db", cfg.Path)
}

func TestDatabaseConfigSetValidatesType(t *testing.T) {
	t.Setenv("DATABASE_TYPE", "oracle")

	require.Error(t, persistence.DatabaseConfigSet.Validate(),
		"validation should reject an unknown database type")
}
