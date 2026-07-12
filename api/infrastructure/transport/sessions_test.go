package transport_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
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
	if err != nil {
		t.Fatalf("persistence.New: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(t.TempDir()))
	if err != nil {
		t.Fatalf("NewKeyStore: %v", err)
	}

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
		if err := users.Create(ctx, u); err != nil {
			t.Fatalf("Create user: %v", err)
		}
	}

	character := &models.Character{Name: "Aria", UserID: player.ID}
	if err := characters.Create(ctx, character); err != nil {
		t.Fatalf("Create character: %v", err)
	}

	router := transport.NewRouter(
		transport.WithHandle("GET /api/sessions", requireAuth(transport.ListSessionsHandler(sessionSvc))),
		transport.WithHandle("POST /api/sessions", requireAuth(transport.CreateSessionHandler(sessionSvc))),
		transport.WithHandle("GET /api/sessions/{id}", requireAuth(transport.GetSessionHandler(sessionSvc))),
		transport.WithHandle("PATCH /api/sessions/{id}", requireAuth(transport.UpdateSessionHandler(sessionSvc))),
		transport.WithHandle(
			"POST /api/sessions/{id}/participants", requireAuth(transport.AddSessionParticipantHandler(sessionSvc)),
		),
		transport.WithHandle(
			"DELETE /api/sessions/{id}/participants/{characterId}",
			requireAuth(transport.RemoveSessionParticipantHandler(sessionSvc)),
		),
		transport.WithHandle(
			"POST /api/sessions/{id}/game-day", requireAuth(transport.AdvanceSessionGameDayHandler(sessionSvc)),
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
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatalf("encoding request: %v", err)
		}
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
	if rec.Code != http.StatusCreated {
		t.Fatalf("create session: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding session: %v", err)
	}

	return created.ID
}

func TestSessions_RequireAuth(t *testing.T) {
	env := newSessionTestEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/sessions", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no token, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSessions_CreateIsGMOnly(t *testing.T) {
	env := newSessionTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/sessions", env.playerToken, map[string]any{"name": "Session One"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for player create, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSessions_ListIsGMOnly(t *testing.T) {
	env := newSessionTestEnv(t)
	env.createSession(t, "Session One")

	rec := env.doJSON(t, http.MethodGet, "/api/sessions", env.playerToken, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for player list, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSessions_AddParticipantAndAdvanceGameDay(t *testing.T) {
	env := newSessionTestEnv(t)
	sessionID := env.createSession(t, "Session One")

	addRec := env.doJSON(t, http.MethodPost, "/api/sessions/"+sessionID+"/participants", env.gmToken,
		map[string]any{"character_id": env.characterID})
	if addRec.Code != http.StatusNoContent {
		t.Fatalf("add participant: expected 204, got %d: %s", addRec.Code, addRec.Body.String())
	}

	advanceRec := env.doJSON(t, http.MethodPost, "/api/sessions/"+sessionID+"/game-day", env.gmToken,
		map[string]any{"delta": 2})
	if advanceRec.Code != http.StatusOK {
		t.Fatalf("advance game day: expected 200, got %d: %s", advanceRec.Code, advanceRec.Body.String())
	}

	getRec := env.doJSON(t, http.MethodGet, "/api/sessions/"+sessionID, env.gmToken, nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var session struct {
		Participants []struct {
			ID string `json:"id"`
		} `json:"participants"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &session); err != nil {
		t.Fatalf("decoding session: %v", err)
	}
	if len(session.Participants) != 1 || session.Participants[0].ID != env.characterID {
		t.Fatalf("participants = %v, want [%s]", session.Participants, env.characterID)
	}

	removeRec := env.doJSON(
		t, http.MethodDelete, "/api/sessions/"+sessionID+"/participants/"+env.characterID, env.gmToken, nil,
	)
	if removeRec.Code != http.StatusNoContent {
		t.Fatalf("remove participant: expected 204, got %d: %s", removeRec.Code, removeRec.Body.String())
	}
}

func TestSessions_AdvanceGameDayIsGMOnly(t *testing.T) {
	env := newSessionTestEnv(t)
	sessionID := env.createSession(t, "Session One")

	rec := env.doJSON(t, http.MethodPost, "/api/sessions/"+sessionID+"/game-day", env.playerToken,
		map[string]any{"delta": 1})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for player game-day advance, got %d: %s", rec.Code, rec.Body.String())
	}
}
