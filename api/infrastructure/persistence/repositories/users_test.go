package repositories_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/stretchr/testify/require"
)

func TestUsers_CreateCountAndGetByEmail(t *testing.T) {
	repo := repositories.NewUsers(newTestDB(t))
	ctx := t.Context()

	count, err := repo.Count(ctx)
	require.NoError(t, err, "Count")
	require.Zero(t, count, "Count on a fresh database")

	user := &models.User{Email: "gm@example.com", PasswordHash: "hash", Role: models.RoleGM}
	require.NoError(t, repo.Create(ctx, user), "Create")
	require.NotEmpty(t, user.ID, "Create should populate the generated ID")

	count, err = repo.Count(ctx)
	require.NoError(t, err, "Count")
	require.EqualValues(t, 1, count, "Count after Create")

	found, err := repo.GetByEmail(ctx, "gm@example.com")
	require.NoError(t, err, "GetByEmail")
	require.Equal(t, user.ID, found.ID)
}

func TestUsers_GetByEmail_NotFound(t *testing.T) {
	repo := repositories.NewUsers(newTestDB(t))

	_, err := repo.GetByEmail(t.Context(), "nobody@example.com")
	require.ErrorIs(t, err, repositories.ErrNotFound)
}

func TestUsers_GetByID(t *testing.T) {
	repo := repositories.NewUsers(newTestDB(t))
	ctx := t.Context()

	user := &models.User{Email: "player@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	require.NoError(t, repo.Create(ctx, user), "Create")

	found, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err, "GetByID")
	require.Equal(t, user.Email, found.Email)

	_, err = repo.GetByID(ctx, "not-an-id")
	require.ErrorIs(t, err, repositories.ErrNotFound, "GetByID(unknown)")
}

func TestUsers_List(t *testing.T) {
	repo := repositories.NewUsers(newTestDB(t))
	ctx := t.Context()

	require.NoError(t, repo.Create(ctx, &models.User{Email: "b@example.com", PasswordHash: "hash", Role: models.RolePlayer}), "Create")
	require.NoError(t, repo.Create(ctx, &models.User{Email: "a@example.com", PasswordHash: "hash", Role: models.RoleGM}), "Create")

	users, err := repo.List(ctx)
	require.NoError(t, err, "List")
	require.Len(t, users, 2)
	require.Equal(t, "a@example.com", users[0].Email, "List should be ordered by email")
	require.Equal(t, "b@example.com", users[1].Email, "List should be ordered by email")
}

func TestUsers_UpdatePasswordHash(t *testing.T) {
	repo := repositories.NewUsers(newTestDB(t))
	ctx := t.Context()

	user := &models.User{Email: "player@example.com", PasswordHash: "old-hash", Role: models.RolePlayer}
	require.NoError(t, repo.Create(ctx, user), "Create")

	require.NoError(t, repo.UpdatePasswordHash(ctx, user.ID, "new-hash"), "UpdatePasswordHash")

	found, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err, "GetByID")
	require.Equal(t, "new-hash", found.PasswordHash)
}
