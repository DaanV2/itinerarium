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

type activityHTTPEnv struct {
	router      *transport.Router
	gmToken     string
	playerToken string
	otherToken  string
	characterID string
}

func newActivityHTTPEnv(t *testing.T) activityHTTPEnv {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err, "persistence.New")
	require.NoError(t, db.Migrate(), "Migrate")

	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(t.TempDir()))
	require.NoError(t, err, "NewKeyStore")

	tokens := authentication.NewTokenService(keys, repositories.NewRevokedTokens(db))
	users := repositories.NewUsers(db)
	characterRepo := repositories.NewCharacters(db)
	knowledgeRepo := repositories.NewKnowledgeRepositories(db)
	groupRepo := repositories.NewGroups(db)

	authSvc := application.NewAuthService(tokens, users)
	charSvc := application.NewCharacterService(characterRepo, users, knowledgeRepo)
	activitySvc := application.NewActivityService(
		repositories.NewActivityEntries(db), charSvc, groupRepo,
		repositories.NewLocationAccesses(db), knowledgeRepo,
	)
	requireAuth := transport.RequireAuth(authSvc)

	ctx := t.Context()

	newUser := func(email string, role models.Role) string {
		u := &models.User{Email: email, PasswordHash: "hash", Role: role}
		require.NoError(t, users.Create(ctx, u), "Create user %s", email)

		token, err := tokens.Issue(u.ID)
		require.NoError(t, err, "Issue(%s)", email)

		return token
	}

	gmToken := newUser("gm@example.com", models.RoleGM)
	playerToken := newUser("player@example.com", models.RolePlayer)
	otherToken := newUser("other@example.com", models.RolePlayer)

	player, err := users.GetByEmail(ctx, "player@example.com")
	require.NoError(t, err, "GetByEmail")

	character := &models.Character{Name: "Aria", UserID: player.ID, CurrentGameDay: 10}
	require.NoError(t, characterRepo.Create(ctx, character), "Create character")

	router := transport.NewRouter(
		transport.WithHandle(
			"GET /api/characters/{id}/activity", requireAuth(handlers.GetCharacterActivityHandler(activitySvc)),
		),
		transport.WithHandle("GET /api/activity", requireAuth(handlers.ListActivityHandler(activitySvc))),
		transport.WithHandle(
			"POST /api/activity/announcements", requireAuth(handlers.AnnounceActivityHandler(activitySvc)),
		),
	)

	return activityHTTPEnv{
		router:      router,
		gmToken:     gmToken,
		playerToken: playerToken,
		otherToken:  otherToken,
		characterID: character.ID,
	}
}

func (e activityHTTPEnv) doJSON(t *testing.T, method, path, token string, payload any) *httptest.ResponseRecorder {
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

func TestCharacterActivity_RequiresAuth(t *testing.T) {
	env := newActivityHTTPEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/characters/"+env.characterID+"/activity", "", nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code, "body: %s", rec.Body.String())
}

func TestCharacterActivity_ForeignCharacterHidden(t *testing.T) {
	env := newActivityHTTPEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/characters/"+env.characterID+"/activity", env.otherToken, nil)
	require.Equal(t, http.StatusNotFound, rec.Code, "body: %s", rec.Body.String())
}

func TestListActivity_PlayerForbidden(t *testing.T) {
	env := newActivityHTTPEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/activity", env.playerToken, nil)
	require.Equal(t, http.StatusForbidden, rec.Code, "body: %s", rec.Body.String())
}

func TestAnnounce_PlayerForbidden_GMCreates_ActorStrippedInPlayerFeed(t *testing.T) {
	env := newActivityHTTPEnv(t)

	announcement := map[string]any{
		"game_day":      5,
		"action":        "stolen",
		"entity_type":   "item",
		"entity_name":   "The Ruby of Vess",
		"actor":         "The Grey Hand",
		"character_ids": []string{env.characterID},
	}

	rec := env.doJSON(t, http.MethodPost, "/api/activity/announcements", env.playerToken, announcement)
	require.Equal(t, http.StatusForbidden, rec.Code, "player announce body: %s", rec.Body.String())

	rec = env.doJSON(t, http.MethodPost, "/api/activity/announcements", env.gmToken, announcement)
	require.Equal(t, http.StatusCreated, rec.Code, "GM announce body: %s", rec.Body.String())

	rec = env.doJSON(t, http.MethodGet, "/api/characters/"+env.characterID+"/activity", env.playerToken, nil)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	// The raw response body must not contain the actor anywhere — the strip
	// happens server-side, never in the client (core domain rule 2).
	require.NotContains(t, rec.Body.String(), "The Grey Hand", "actor leaked into player response")

	var feed []struct {
		EntityName string `json:"entity_name"`
		Announced  bool   `json:"announced"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &feed), "decoding feed")
	require.Len(t, feed, 1, "want only the announced theft")
	require.Equal(t, "The Ruby of Vess", feed[0].EntityName)
	require.True(t, feed[0].Announced)
}
