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

type sessionTestEnv struct {
	router      *transport.Router
	gmToken     string
	playerToken string
	characterID string
}

func newSessionTestEnv(t *testing.T) sessionTestEnv {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err, "persistence.New")
	require.NoError(t, db.Migrate(), "Migrate")

	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(t.TempDir()))
	require.NoError(t, err, "NewKeyStore")

	tokens := authentication.NewTokenService(keys, repositories.NewRevokedTokens(db))
	users := repositories.NewUsers(db)
	characters := repositories.NewCharacters(db)
	authSvc := application.NewAuthService(tokens, users)
	characterSvc := application.NewCharacterService(characters, users, repositories.NewKnowledgeRepositories(db))
	sessionSvc := application.NewSessionService(repositories.NewSessions(db), characterSvc)
	requireAuth := transport.RequireAuth(authSvc)

	ctx := t.Context()
	gm := &models.User{Email: "gm@example.com", PasswordHash: "hash", Role: models.RoleGM}
	player := &models.User{Email: "player@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	for _, u := range []*models.User{gm, player} {
		require.NoError(t, users.Create(ctx, u), "Create user")
	}

	character := &models.Character{Name: "Aria", UserID: player.ID}
	require.NoError(t, characters.Create(ctx, character), "Create character")

	router := transport.NewRouter(
		transport.WithHandle("GET /api/sessions", requireAuth(handlers.ListSessionsHandler(sessionSvc))),
		transport.WithHandle("POST /api/sessions", requireAuth(handlers.CreateSessionHandler(sessionSvc))),
		transport.WithHandle("GET /api/sessions/{id}", requireAuth(handlers.GetSessionHandler(sessionSvc))),
		transport.WithHandle("PATCH /api/sessions/{id}", requireAuth(handlers.UpdateSessionHandler(sessionSvc))),
		transport.WithHandle(
			"POST /api/sessions/{id}/participants", requireAuth(handlers.AddSessionParticipantHandler(sessionSvc)),
		),
		transport.WithHandle(
			"DELETE /api/sessions/{id}/participants/{characterId}",
			requireAuth(handlers.RemoveSessionParticipantHandler(sessionSvc)),
		),
		transport.WithHandle(
			"POST /api/sessions/{id}/game-day", requireAuth(handlers.AdvanceSessionGameDayHandler(sessionSvc)),
		),
	)

	return sessionTestEnv{
		router:      router,
		gmToken:     issueToken(t, tokens, gm.ID),
		playerToken: issueToken(t, tokens, player.ID),
		characterID: character.ID,
	}
}

func (e sessionTestEnv) doJSON(t *testing.T, method, path, token string, payload any) *httptest.ResponseRecorder {
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

func (e sessionTestEnv) createSession(t *testing.T, name string) string {
	t.Helper()

	rec := e.doJSON(t, http.MethodPost, "/api/sessions", e.gmToken, map[string]any{"name": name})
	require.Equal(t, http.StatusCreated, rec.Code, "create session body: %s", rec.Body.String())

	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created), "decoding session")

	return created.ID
}

func TestSessions_RequireAuth(t *testing.T) {
	env := newSessionTestEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/sessions", "", nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code, "body: %s", rec.Body.String())
}

func TestSessions_CreateIsGMOnly(t *testing.T) {
	env := newSessionTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/sessions", env.playerToken, map[string]any{"name": "Session One"})
	require.Equal(t, http.StatusForbidden, rec.Code, "player create body: %s", rec.Body.String())
}

func TestSessions_ListIsGMOnly(t *testing.T) {
	env := newSessionTestEnv(t)
	env.createSession(t, "Session One")

	rec := env.doJSON(t, http.MethodGet, "/api/sessions", env.playerToken, nil)
	require.Equal(t, http.StatusForbidden, rec.Code, "player list body: %s", rec.Body.String())
}

func TestSessions_AddParticipantAndAdvanceGameDay(t *testing.T) {
	env := newSessionTestEnv(t)
	sessionID := env.createSession(t, "Session One")

	addRec := env.doJSON(t, http.MethodPost, "/api/sessions/"+sessionID+"/participants", env.gmToken,
		map[string]any{"character_id": env.characterID})
	require.Equal(t, http.StatusNoContent, addRec.Code, "add participant body: %s", addRec.Body.String())

	advanceRec := env.doJSON(t, http.MethodPost, "/api/sessions/"+sessionID+"/game-day", env.gmToken,
		map[string]any{"delta": 2})
	require.Equal(t, http.StatusOK, advanceRec.Code, "advance game day body: %s", advanceRec.Body.String())

	getRec := env.doJSON(t, http.MethodGet, "/api/sessions/"+sessionID, env.gmToken, nil)
	require.Equal(t, http.StatusOK, getRec.Code, "get body: %s", getRec.Body.String())

	var session struct {
		Participants []struct {
			ID string `json:"id"`
		} `json:"participants"`
	}
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &session), "decoding session")
	require.Len(t, session.Participants, 1)
	require.Equal(t, env.characterID, session.Participants[0].ID)

	removeRec := env.doJSON(
		t, http.MethodDelete, "/api/sessions/"+sessionID+"/participants/"+env.characterID, env.gmToken, nil,
	)
	require.Equal(t, http.StatusNoContent, removeRec.Code, "remove participant body: %s", removeRec.Body.String())
}

func TestSessions_AdvanceGameDayIsGMOnly(t *testing.T) {
	env := newSessionTestEnv(t)
	sessionID := env.createSession(t, "Session One")

	rec := env.doJSON(t, http.MethodPost, "/api/sessions/"+sessionID+"/game-day", env.playerToken,
		map[string]any{"delta": 1})
	require.Equal(t, http.StatusForbidden, rec.Code, "player game-day advance body: %s", rec.Body.String())
}
