package repositories_test

import (
	"testing"
	"time"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

func newTestDB(t *testing.T) *persistence.Database {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	if err != nil {
		t.Fatalf("persistence.New: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	return db
}

func TestRevokedTokens_RevokeAndIsRevoked(t *testing.T) {
	repo := repositories.NewRevokedTokens(newTestDB(t))
	ctx := t.Context()

	revoked, err := repo.IsRevoked(ctx, "jti-1")
	if err != nil {
		t.Fatalf("IsRevoked: %v", err)
	}
	if revoked {
		t.Fatal("expected an unknown JTI to not be revoked")
	}

	if err := repo.Revoke(ctx, "jti-1", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	revoked, err = repo.IsRevoked(ctx, "jti-1")
	if err != nil {
		t.Fatalf("IsRevoked: %v", err)
	}
	if !revoked {
		t.Fatal("expected the revoked JTI to report as revoked")
	}
}

func TestRevokedTokens_RevokeIsIdempotent(t *testing.T) {
	repo := repositories.NewRevokedTokens(newTestDB(t))
	ctx := t.Context()

	if err := repo.Revoke(ctx, "jti-1", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("Revoke (first): %v", err)
	}
	if err := repo.Revoke(ctx, "jti-1", time.Now().Add(2*time.Hour)); err != nil {
		t.Fatalf("Revoke (second): %v", err)
	}
}
