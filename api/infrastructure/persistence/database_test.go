package persistence_test

import (
	"context"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryDatabaseMigratesAndShutsDown(t *testing.T) {
	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err, "New")

	require.NoError(t, db.Migrate(), "Migrate")
	require.NoError(t, db.Shutdown(context.Background()), "Shutdown")
}

func TestNewRejectsUnsupportedType(t *testing.T) {
	_, err := persistence.New(persistence.WithType("bogus"))
	require.Error(t, err, "New should reject an unsupported database type")
}

func TestDBTypeValid(t *testing.T) {
	for _, tt := range []struct {
		dbType persistence.DBType
		want   bool
	}{
		{persistence.SQLite, true},
		{persistence.InMemory, true},
		{persistence.PostgreSQL, true},
		{persistence.MySQL, true},
		{persistence.DBType("bogus"), false},
	} {
		assert.Equal(t, tt.want, tt.dbType.Valid(), "DBType(%q).Valid()", tt.dbType)
	}
}
