package repositories_test

import (
	"testing"
	"time"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *persistence.Database {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err, "persistence.New")
	require.NoError(t, db.Migrate(), "Migrate")

	return db
}

func TestRevokedTokens_RevokeAndIsRevoked(t *testing.T) {
	repo := repositories.NewRevokedTokens(newTestDB(t))
	ctx := t.Context()

	revoked, err := repo.IsRevoked(ctx, "jti-1")
	require.NoError(t, err, "IsRevoked")
	require.False(t, revoked, "an unknown JTI should not be revoked")

	require.NoError(t, repo.Revoke(ctx, "jti-1", time.Now().Add(time.Hour)), "Revoke")

	revoked, err = repo.IsRevoked(ctx, "jti-1")
	require.NoError(t, err, "IsRevoked")
	require.True(t, revoked, "the revoked JTI should report as revoked")
}

func TestRevokedTokens_RevokeIsIdempotent(t *testing.T) {
	repo := repositories.NewRevokedTokens(newTestDB(t))
	ctx := t.Context()

	require.NoError(t, repo.Revoke(ctx, "jti-1", time.Now().Add(time.Hour)), "Revoke (first)")
	require.NoError(t, repo.Revoke(ctx, "jti-1", time.Now().Add(2*time.Hour)), "Revoke (second)")
}
