package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/handlers"
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/DaanV2/itinerarium/api/transport"
	"github.com/stretchr/testify/require"
)

type journalTestEnv struct {
	router      *transport.Router
	gmToken     string
	playerToken string
	otherToken  string
	characterID string
}

func newJournalTestEnv(t *testing.T) journalTestEnv {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err, "persistence.New")
	require.NoError(t, db.Migrate(), "Migrate")

	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(t.TempDir()))
	require.NoError(t, err, "NewKeyStore")

	tokens := authentication.NewTokenService(keys, repositories.NewRevokedTokens(db))
	users := repositories.NewUsers(db)
	characters := repositories.NewCharacters(db)
	groups := repositories.NewGroups(db)
	journalEntries := repositories.NewJournalEntries(db)
	knowledgeRepos := repositories.NewKnowledgeRepositories(db)
	authSvc := application.NewAuthService(tokens, users)
	repositoryService := application.NewRepositoryService(knowledgeRepos, groups, characters)
	documentSvc := application.NewDocumentService(
		repositories.NewDocuments(db), repositoryService, characters, groups, repositories.NewDocumentShares(db),
	)
	journalSvc := application.NewJournalEntryService(journalEntries, characters, documentSvc, knowledgeRepos)
	requireAuth := transport.RequireAuth(authSvc)

	ctx := t.Context()

	gm := &models.User{Email: "gm@example.com", PasswordHash: "hash", Role: models.RoleGM}
	require.NoError(t, users.Create(ctx, gm), "Create gm")

	player := &models.User{Email: "player@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	require.NoError(t, users.Create(ctx, player), "Create player")

	other := &models.User{Email: "other@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	require.NoError(t, users.Create(ctx, other), "Create other")

	gmToken, err := tokens.Issue(gm.ID)
	require.NoError(t, err, "Issue(gm)")

	playerToken, err := tokens.Issue(player.ID)
	require.NoError(t, err, "Issue(player)")

	otherToken, err := tokens.Issue(other.ID)
	require.NoError(t, err, "Issue(other)")

	character := &models.Character{Name: "Aria", UserID: player.ID}
	require.NoError(t, characters.Create(ctx, character), "Create character")

	router := transport.NewRouter(
		transport.WithHandle(
			"GET /api/characters/{id}/journal", requireAuth(handlers.ListJournalEntriesHandler(journalSvc)),
		),
		transport.WithHandle(
			"POST /api/characters/{id}/journal", requireAuth(handlers.CreateJournalEntryHandler(journalSvc)),
		),
		transport.WithHandle(
			"GET /api/characters/{id}/journal/{entryId}", requireAuth(handlers.GetJournalEntryHandler(journalSvc)),
		),
		transport.WithHandle(
			"PATCH /api/characters/{id}/journal/{entryId}",
			requireAuth(handlers.UpdateJournalEntryHandler(journalSvc)),
		),
		transport.WithHandle(
			"POST /api/characters/{id}/journal/{entryId}/convert",
			requireAuth(handlers.ConvertJournalEntryHandler(journalSvc)),
		),
	)

	return journalTestEnv{
		router:      router,
		gmToken:     gmToken,
		playerToken: playerToken,
		otherToken:  otherToken,
		characterID: character.ID,
	}
}

func (e journalTestEnv) doJSON(t *testing.T, method, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	if payload != nil {
		require.NoError(t, json.NewEncoder(&body).Encode(payload), "encoding request")
	}

	req := httptest.NewRequestWithContext(t.Context(), method, path, &body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)

	return rec
}

func TestCreateJournalEntry_RequiresAuth(t *testing.T) {
	env := newJournalTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/journal", "",
		map[string]string{"content": "Dear diary"})
	require.Equal(t, http.StatusUnauthorized, rec.Code, "body: %s", rec.Body.String())
}

func TestCreateJournalEntry_OwnerCanWrite(t *testing.T) {
	env := newJournalTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/journal", env.playerToken,
		map[string]string{"content": "Dear diary"})
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	var created struct {
		Content string `json:"content"`
		GameDay int    `json:"game_day"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created), "decoding body")
	require.Equal(t, "Dear diary", created.Content)
}

func TestCreateJournalEntry_OtherPlayerCannotWriteForForeignCharacter(t *testing.T) {
	env := newJournalTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/journal", env.otherToken,
		map[string]string{"content": "Not mine to write"})
	require.Equal(t, http.StatusNotFound, rec.Code, "body: %s", rec.Body.String())
}

func TestListJournalEntries_OtherPlayerHidden(t *testing.T) {
	env := newJournalTestEnv(t)

	env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/journal", env.playerToken,
		map[string]string{"content": "Secret entry"})

	rec := env.doJSON(t, http.MethodGet, "/api/characters/"+env.characterID+"/journal", env.otherToken, nil)
	require.Equal(t, http.StatusNotFound, rec.Code, "body: %s", rec.Body.String())
}

func TestListJournalEntries_GMCanReadAnyCharacter(t *testing.T) {
	env := newJournalTestEnv(t)

	env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/journal", env.playerToken,
		map[string]string{"content": "Entry one"})

	rec := env.doJSON(t, http.MethodGet, "/api/characters/"+env.characterID+"/journal", env.gmToken, nil)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var list []struct{ Content string }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list), "decoding body")
	require.Len(t, list, 1)
}

func TestGetJournalEntry_OtherPlayerHidden(t *testing.T) {
	env := newJournalTestEnv(t)

	createRec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/journal", env.playerToken,
		map[string]string{"content": "Secret entry"})

	var created struct{ ID string }
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created), "decoding body")

	rec := env.doJSON(
		t, http.MethodGet, "/api/characters/"+env.characterID+"/journal/"+created.ID, env.otherToken, nil,
	)
	require.Equal(t, http.StatusNotFound, rec.Code, "body: %s", rec.Body.String())
}

func TestUpdateJournalEntry_OwnerCanEdit(t *testing.T) {
	env := newJournalTestEnv(t)

	createRec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/journal", env.playerToken,
		map[string]string{"content": "Original"})

	var created struct{ ID string }
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created), "decoding body")

	rec := env.doJSON(
		t, http.MethodPatch, "/api/characters/"+env.characterID+"/journal/"+created.ID, env.playerToken,
		map[string]string{"content": "Revised"},
	)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var updated struct{ Content string }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated), "decoding body")
	require.Equal(t, "Revised", updated.Content)
}

func TestUpdateJournalEntry_OtherPlayerCannotEdit(t *testing.T) {
	env := newJournalTestEnv(t)

	createRec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/journal", env.playerToken,
		map[string]string{"content": "Original"})

	var created struct{ ID string }
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created), "decoding body")

	rec := env.doJSON(
		t, http.MethodPatch, "/api/characters/"+env.characterID+"/journal/"+created.ID, env.otherToken,
		map[string]string{"content": "Hijacked"},
	)
	require.Equal(t, http.StatusNotFound, rec.Code, "body: %s", rec.Body.String())
}

func TestConvertJournalEntry_OwnerCreatesDocument(t *testing.T) {
	env := newJournalTestEnv(t)

	createRec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/journal", env.playerToken,
		map[string]string{"content": "Dear diary, today I met a dragon."})

	var created struct{ ID string }
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created), "decoding body")

	rec := env.doJSON(
		t, http.MethodPost, "/api/characters/"+env.characterID+"/journal/"+created.ID+"/convert", env.playerToken, nil,
	)
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	var doc struct {
		Title    string `json:"title"`
		Sections []struct{ Content string }
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &doc), "decoding body")
	require.Len(t, doc.Sections, 1)
	require.Equal(t, "Dear diary, today I met a dragon.", doc.Sections[0].Content)

	// The journal entry itself is unaffected.
	getRec := env.doJSON(
		t, http.MethodGet, "/api/characters/"+env.characterID+"/journal/"+created.ID, env.playerToken, nil,
	)
	require.Equal(t, http.StatusOK, getRec.Code, "body: %s", getRec.Body.String())
}

func TestConvertJournalEntry_OtherPlayerCannotConvert(t *testing.T) {
	env := newJournalTestEnv(t)

	createRec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/journal", env.playerToken,
		map[string]string{"content": "Secret entry"})

	var created struct{ ID string }
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created), "decoding body")

	rec := env.doJSON(
		t, http.MethodPost, "/api/characters/"+env.characterID+"/journal/"+created.ID+"/convert", env.otherToken, nil,
	)
	require.Equal(t, http.StatusNotFound, rec.Code, "body: %s", rec.Body.String())
}
