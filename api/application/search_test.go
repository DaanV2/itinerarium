package application_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resultPaths projects search results onto their document paths, keeping
// assertions readable.
func resultPaths(results []application.SearchResult) []string {
	paths := make([]string, len(results))
	for i := range results {
		paths[i] = results[i].Document.Path
	}

	return paths
}

func TestDocumentService_Search_EmptyQueryRejected(t *testing.T) {
	env := newDocumentTestEnv(t)

	_, err := env.docs.Search(t.Context(), gmRequester, "   ")
	require.ErrorIs(t, err, application.ErrInvalidQuery)
}

func TestDocumentService_Search_GMMatchesEveryField(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path: "lore/dragon-history", Title: "Dragon History",
	})
	mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path: "factions/guild", Title: "The Guild", Tags: []string{"dragons", "politics"},
	})
	mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path:     "places/lair",
		Title:    "The Lair",
		Sections: []application.DocumentSectionInput{{Content: "Home of the elder dragon Vyranox."}},
	})
	mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path: "places/harbor", Title: "Harbor",
		Sections: []application.DocumentSectionInput{{Content: "Ships and sailors."}},
	})

	results, err := env.docs.Search(ctx, gmRequester, "dragon")
	require.NoError(t, err)

	assert.ElementsMatch(t,
		[]string{"lore/dragon-history", "factions/guild", "places/lair"},
		resultPaths(results),
	)

	for _, r := range results {
		switch r.Document.Path {
		case "lore/dragon-history":
			assert.Contains(t, r.MatchedIn, application.SearchMatchTitle)
			assert.Contains(t, r.MatchedIn, application.SearchMatchPath)
		case "factions/guild":
			assert.Equal(t, []string{application.SearchMatchTags}, r.MatchedIn)
		case "places/lair":
			assert.Equal(t, []string{application.SearchMatchContent}, r.MatchedIn)
			assert.Contains(t, r.Snippet, "elder dragon Vyranox")
		}
	}
}

func TestDocumentService_Search_GMMatchesGMOnlySections(t *testing.T) {
	env := newDocumentTestEnv(t)
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path:     "lore/secret",
		Title:    "Innocent Title",
		Sections: []application.DocumentSectionInput{{Content: "The lich king sleeps here.", GMOnly: true}},
	})

	results, err := env.docs.Search(t.Context(), gmRequester, "lich king")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, []string{application.SearchMatchContent}, results[0].MatchedIn)
}

func TestDocumentService_Search_PlayerGameDayGate(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	day := 5
	mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path: "lore/prophecy", Title: "The Prophecy", SharedOnGameDay: &day,
		Sections: []application.DocumentSectionInput{{Content: "A dragon will rise."}},
	})

	// Day 0: nothing — not even a hit count.
	results, err := env.docs.Search(ctx, playerRequester, "dragon")
	require.NoError(t, err)
	assert.Empty(t, results)

	// Day reached: the document surfaces.
	env.setGameDay(t, character.ID, 5)
	results, err = env.docs.Search(ctx, playerRequester, "dragon")
	require.NoError(t, err)
	assert.Equal(t, []string{"lore/prophecy"}, resultPaths(results))

	// Rewind: it disappears again.
	env.setGameDay(t, character.ID, 2)
	results, err = env.docs.Search(ctx, playerRequester, "dragon")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestDocumentService_Search_PlayerCannotReachOtherCharacterRepository(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()

	_, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	other := fakeRequester{id: "player-2", gm: false}
	otherCharacter, err := env.characters.Create(ctx, other, "", "Beren")
	require.NoError(t, err)

	otherRepo := env.findRepository(t, models.RepositoryTypeCharacter, otherCharacter.ID)
	mustCreateDocument(t, env, gmRequester, otherRepo.ID, &application.CreateDocumentInput{
		Path: "notes/dragon-sighting", Title: "Dragon Sighting",
	})

	results, err := env.docs.Search(ctx, playerRequester, "dragon")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestDocumentService_Search_PlayerGroupRepositoryNeedsMembership(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()

	member, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	outsider := fakeRequester{id: "player-2", gm: false}
	_, err = env.characters.Create(ctx, outsider, "", "Beren")
	require.NoError(t, err)

	group, err := env.groups.Create(ctx, gmRequester, "Thieves Guild", models.GroupTypeOrganization, "")
	require.NoError(t, err)
	err = env.groups.Join(ctx, playerRequester, group.ID, member.ID)
	require.NoError(t, err)

	groupRepo := env.findRepository(t, models.RepositoryTypeGroup, group.ID)
	mustCreateDocument(t, env, gmRequester, groupRepo.ID, &application.CreateDocumentInput{
		Path: "heists/dragon-vault", Title: "Dragon Vault Heist",
	})

	results, err := env.docs.Search(ctx, playerRequester, "dragon")
	require.NoError(t, err)
	assert.Equal(t, []string{"heists/dragon-vault"}, resultPaths(results))

	results, err = env.docs.Search(ctx, outsider, "dragon")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestDocumentService_Search_GMOnlyContentInvisibleToPlayers(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	_, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	// Matchable only through its GM-only section.
	mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path:  "lore/secret",
		Title: "Innocent Title",
		Sections: []application.DocumentSectionInput{
			{Content: "Public knowledge."},
			{Content: "The lich king sleeps here.", GMOnly: true},
		},
	})

	results, err := env.docs.Search(ctx, playerRequester, "lich king")
	require.NoError(t, err)
	assert.Empty(t, results, "GM-only content must not be searchable for players")

	// The same document found through a visible field must come back with the
	// GM-only section stripped.
	results, err = env.docs.Search(ctx, playerRequester, "innocent")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Len(t, results[0].Document.Sections, 1)
	assert.Equal(t, "Public knowledge.", results[0].Document.Sections[0].Content)
}

func TestDocumentService_Search_DirectShareUnlocksDocument(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()

	character, err := env.characters.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	other := fakeRequester{id: "player-2", gm: false}
	otherCharacter, err := env.characters.Create(ctx, other, "", "Beren")
	require.NoError(t, err)

	otherRepo := env.findRepository(t, models.RepositoryTypeCharacter, otherCharacter.ID)
	view := mustCreateDocument(t, env, gmRequester, otherRepo.ID, &application.CreateDocumentInput{
		Path: "notes/dragon-map", Title: "Dragon Map",
	})

	_, err = env.docs.ShareWithCharacter(ctx, gmRequester, view.Document.ID, character.ID, 3)
	require.NoError(t, err)

	// Share day not reached yet.
	results, err := env.docs.Search(ctx, playerRequester, "dragon")
	require.NoError(t, err)
	assert.Empty(t, results)

	env.setGameDay(t, character.ID, 3)
	results, err = env.docs.Search(ctx, playerRequester, "dragon")
	require.NoError(t, err)
	assert.Equal(t, []string{"notes/dragon-map"}, resultPaths(results))
}

func TestDocumentService_Search_LikeWildcardsAreLiteral(t *testing.T) {
	env := newDocumentTestEnv(t)
	ctx := t.Context()
	general := env.findRepository(t, models.RepositoryTypeGeneral, "")

	mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path: "shop/sale", Title: "Sale",
		Sections: []application.DocumentSectionInput{{Content: "Everything 50% off today."}},
	})
	mustCreateDocument(t, env, gmRequester, general.ID, &application.CreateDocumentInput{
		Path: "shop/stock", Title: "Stock",
		Sections: []application.DocumentSectionInput{{Content: "Fifty items in stock."}},
	})

	results, err := env.docs.Search(ctx, gmRequester, "50%")
	require.NoError(t, err)
	assert.Equal(t, []string{"shop/sale"}, resultPaths(results))

	results, err = env.docs.Search(ctx, gmRequester, "_")
	require.NoError(t, err)
	assert.Empty(t, results, "underscore must not act as a single-character wildcard")
}
