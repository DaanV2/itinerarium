package handlers_test

import (
	"net/http"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createSecondPlayerCharacterRepo provisions a character owned by a different
// user and returns that character's repository ID — a target the env's player
// must not be able to reach.
func createSecondPlayerCharacterRepo(t *testing.T, env *documentsTransportEnv) string {
	t.Helper()

	character := &models.Character{Name: "Beren", UserID: "someone-else"}
	err := env.characters.Create(t.Context(), character)
	require.NoError(t, err)

	repo, err := env.knowledgeRepos.EnsureForCharacter(t.Context(), character.ID)
	require.NoError(t, err)

	return repo.ID
}

func TestSearchRoute_RequiresQueryAndAuth(t *testing.T) {
	env := newDocumentsTransportEnv(t)

	rec := env.do(t, http.MethodGet, "/api/search?q=dragon", "", nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = env.do(t, http.MethodGet, "/api/search", env.gmToken, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSearchRoute_PlayerPayloadNeverContainsGMOnlyOrGatedContent(t *testing.T) {
	env := newDocumentsTransportEnv(t)

	// Matchable by players through its title; carries a GM-only secret.
	env.createDocument(t, map[string]any{
		"path": "npcs/dragon-duke",
		"sections": []map[string]any{
			{"content": "The duke rules the city."},
			{"content": "He is secretly a vampire dragon.", "gm_only": true},
		},
	})
	// Gated behind a future game day.
	env.createDocument(t, map[string]any{
		"path":               "reveals/dragon-betrayal",
		"shared_on_game_day": 5,
	})

	rec := env.do(t, http.MethodGet, "/api/search?q=dragon", env.playerToken, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	payload := rec.Body.String()
	assert.Contains(t, payload, "dragon-duke")
	assert.NotContains(t, payload, "vampire", "GM-only content leaked into player search payload")
	assert.NotContains(t, payload, "dragon-betrayal", "gated document leaked into player search results")

	// The GM sees both, GM-only content included in matching.
	rec = env.do(t, http.MethodGet, "/api/search?q=vampire", env.gmToken, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "dragon-duke")

	rec = env.do(t, http.MethodGet, "/api/search?q=dragon", env.gmToken, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "dragon-betrayal")
}

func TestImportRoute_ImportsAndReportsCollisions(t *testing.T) {
	env := newDocumentsTransportEnv(t)

	body := map[string]any{
		"repository_id": env.generalID,
		"files": []map[string]any{
			{"path": "lore/origins.md", "markdown": "---\ntitle: Origins\ntags: [lore]\n---\nIn the beginning."},
		},
	}

	rec := env.do(t, http.MethodPost, "/api/import/obsidian", env.gmToken, body)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"imported"`)

	// The same batch again warns per file instead of failing the request.
	rec = env.do(t, http.MethodPost, "/api/import/obsidian", env.gmToken, body)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"collision"`)

	// The imported document is now searchable.
	rec = env.do(t, http.MethodGet, "/api/search?q=beginning", env.gmToken, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "lore/origins")
}

func TestImportRoute_PlayerCannotTargetInaccessibleRepository(t *testing.T) {
	env := newDocumentsTransportEnv(t)

	characterRepoID := env.findRepositoryID(t, models.RepositoryTypeCharacter, env.characterID)
	require.NotEmpty(t, characterRepoID)

	// Into their own repository: fine.
	rec := env.do(t, http.MethodPost, "/api/import/obsidian", env.playerToken, map[string]any{
		"repository_id": characterRepoID,
		"files":         []map[string]any{{"path": "diary/day-one.md", "markdown": "My first day."}},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"imported"`)

	// Template repository is writable by everyone who sees it; but another
	// player's character repository must read as 404, never 403.
	otherRepoID := createSecondPlayerCharacterRepo(t, &env)
	rec = env.do(t, http.MethodPost, "/api/import/obsidian", env.playerToken, map[string]any{
		"repository_id": otherRepoID,
		"files":         []map[string]any{{"path": "sneaky.md", "markdown": "Body."}},
	})
	require.Equal(t, http.StatusNotFound, rec.Code)
}
