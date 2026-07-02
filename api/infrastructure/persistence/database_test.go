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
