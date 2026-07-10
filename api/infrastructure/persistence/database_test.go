package persistence_test

import (
	"context"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
)

func TestInMemoryDatabaseMigratesAndShutsDown(t *testing.T) {
	db, err := persistence.New(persistence.WithInMemory())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	if err := db.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestNewRejectsUnsupportedType(t *testing.T) {
	if _, err := persistence.New(persistence.WithType("bogus")); err == nil {
		t.Fatal("expected New to reject an unsupported database type")
	}
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
		if got := tt.dbType.Valid(); got != tt.want {
			t.Errorf("DBType(%q).Valid() = %v, want %v", tt.dbType, got, tt.want)
		}
	}
}
