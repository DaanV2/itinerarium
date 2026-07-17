package repositories_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/stretchr/testify/require"
)

func TestKnowledgeRepositories_EnsureGeneralAndTemplate_AreSingletonsAndIdempotent(t *testing.T) {
	repo := repositories.NewKnowledgeRepositories(newTestDB(t))
	ctx := t.Context()

	general1, err := repo.EnsureGeneral(ctx)
	require.NoError(t, err, "EnsureGeneral")

	general2, err := repo.EnsureGeneral(ctx)
	require.NoError(t, err, "EnsureGeneral (second)")
	require.Equal(t, general1.ID, general2.ID, "EnsureGeneral must not create a second row")

	template, err := repo.EnsureTemplate(ctx)
	require.NoError(t, err, "EnsureTemplate")
	require.NotEqual(t, general1.ID, template.ID, "template repository must not reuse the general repository's ID")

	all, err := repo.List(ctx)
	require.NoError(t, err, "List")
	require.Len(t, all, 2)
}

func TestKnowledgeRepositories_EnsureForGroup_OnePerGroup(t *testing.T) {
	repo := repositories.NewKnowledgeRepositories(newTestDB(t))
	ctx := t.Context()

	first, err := repo.EnsureForGroup(ctx, "group-1")
	require.NoError(t, err, "EnsureForGroup")
	require.Equal(t, models.RepositoryTypeGroup, first.Type)
	require.NotNil(t, first.GroupID)
	require.Equal(t, "group-1", *first.GroupID)

	second, err := repo.EnsureForGroup(ctx, "group-1")
	require.NoError(t, err, "EnsureForGroup (second)")
	require.Equal(t, first.ID, second.ID, "EnsureForGroup must not create a second row for the same group")

	other, err := repo.EnsureForGroup(ctx, "group-2")
	require.NoError(t, err, "EnsureForGroup(group-2)")
	require.NotEqual(t, first.ID, other.ID, "two different groups must not share a repository")
}

func TestKnowledgeRepositories_EnsureForCharacter_OnePerCharacter(t *testing.T) {
	repo := repositories.NewKnowledgeRepositories(newTestDB(t))
	ctx := t.Context()

	first, err := repo.EnsureForCharacter(ctx, "char-1")
	require.NoError(t, err, "EnsureForCharacter")
	require.Equal(t, models.RepositoryTypeCharacter, first.Type)
	require.NotNil(t, first.CharacterID)
	require.Equal(t, "char-1", *first.CharacterID)

	second, err := repo.EnsureForCharacter(ctx, "char-1")
	require.NoError(t, err, "EnsureForCharacter (second)")
	require.Equal(t, first.ID, second.ID, "EnsureForCharacter must not create a second row for the same character")
}

func TestKnowledgeRepositories_GetByID_NotFound(t *testing.T) {
	repo := repositories.NewKnowledgeRepositories(newTestDB(t))

	_, err := repo.GetByID(t.Context(), "does-not-exist")
	require.ErrorIs(t, err, repositories.ErrNotFound, "GetByID(missing)")
}

func TestKnowledgeRepositories_ListVisible(t *testing.T) {
	repo := repositories.NewKnowledgeRepositories(newTestDB(t))
	ctx := t.Context()

	_, err := repo.EnsureGeneral(ctx)
	require.NoError(t, err, "EnsureGeneral")
	_, err = repo.EnsureTemplate(ctx)
	require.NoError(t, err, "EnsureTemplate")
	_, err = repo.EnsureForCharacter(ctx, "char-1")
	require.NoError(t, err, "EnsureForCharacter")
	_, err = repo.EnsureForCharacter(ctx, "char-2")
	require.NoError(t, err, "EnsureForCharacter(char-2)")
	_, err = repo.EnsureForGroup(ctx, "group-1")
	require.NoError(t, err, "EnsureForGroup")

	visible, err := repo.ListVisible(ctx, []string{"char-1"}, []string{"group-1"})
	require.NoError(t, err, "ListVisible")

	// general + template + char-1's own repository + group-1's repository —
	// char-2's repository must not appear.
	require.Len(t, visible, 4)
	for _, r := range visible {
		if r.CharacterID != nil {
			require.NotEqual(t, "char-2", *r.CharacterID, "ListVisible leaked another character's repository")
		}
	}
}
