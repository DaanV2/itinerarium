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
		if err := users.Create(ctx, u); err != nil {
			t.Fatalf("Create user %s: %v", email, err)
		}

		token, err := tokens.Issue(u.ID)
		if err != nil {
			t.Fatalf("Issue(%s): %v", email, err)
		}

		return token
	}

	gmToken := newUser("gm@example.com", models.RoleGM)
	playerToken := newUser("player@example.com", models.RolePlayer)
	otherToken := newUser("other@example.com", models.RolePlayer)

	player, err := users.GetByEmail(ctx, "player@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}

	character := &models.Character{Name: "Aria", UserID: player.ID, CurrentGameDay: 10}
	if err := characterRepo.Create(ctx, character); err != nil {
		t.Fatalf("Create character: %v", err)
	}

	router := transport.NewRouter(
		transport.WithHandle(
			"GET /api/characters/{id}/activity", requireAuth(transport.GetCharacterActivityHandler(activitySvc)),
		),
		transport.WithHandle("GET /api/activity", requireAuth(transport.ListActivityHandler(activitySvc))),
		transport.WithHandle(
			"POST /api/activity/announcements", requireAuth(transport.AnnounceActivityHandler(activitySvc)),
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

func TestCharacterActivity_RequiresAuth(t *testing.T) {
	env := newActivityHTTPEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/characters/"+env.characterID+"/activity", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no token, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCharacterActivity_ForeignCharacterHidden(t *testing.T) {
	env := newActivityHTTPEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/characters/"+env.characterID+"/activity", env.otherToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListActivity_PlayerForbidden(t *testing.T) {
	env := newActivityHTTPEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/activity", env.playerToken, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
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

	if rec := env.doJSON(
		t, http.MethodPost, "/api/activity/announcements", env.playerToken, announcement,
	); rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for player announce, got %d: %s", rec.Code, rec.Body.String())
	}

	if rec := env.doJSON(
		t, http.MethodPost, "/api/activity/announcements", env.gmToken, announcement,
	); rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for GM announce, got %d: %s", rec.Code, rec.Body.String())
	}

	rec := env.doJSON(t, http.MethodGet, "/api/characters/"+env.characterID+"/activity", env.playerToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// The raw response body must not contain the actor anywhere — the strip
	// happens server-side, never in the client (core domain rule 2).
	if bytes.Contains(rec.Body.Bytes(), []byte("The Grey Hand")) {
		t.Fatalf("actor leaked into player response: %s", rec.Body.String())
	}

	var feed []struct {
		EntityName string `json:"entity_name"`
		Announced  bool   `json:"announced"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &feed); err != nil {
		t.Fatalf("decoding feed: %v", err)
	}
	if len(feed) != 1 || feed[0].EntityName != "The Ruby of Vess" || !feed[0].Announced {
		t.Fatalf("feed = %+v, want the announced theft", feed)
	}
}
