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
	journalEntries := repositories.NewJournalEntries(db)
	authSvc := application.NewAuthService(tokens, users)
	journalSvc := application.NewJournalEntryService(journalEntries, characters)
	requireAuth := transport.RequireAuth(authSvc)

	ctx := t.Context()

	gm := &models.User{Email: "gm@example.com", PasswordHash: "hash", Role: models.RoleGM}
	if err := users.Create(ctx, gm); err != nil {
		t.Fatalf("Create gm: %v", err)
	}

	player := &models.User{Email: "player@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	if err := users.Create(ctx, player); err != nil {
		t.Fatalf("Create player: %v", err)
	}

	other := &models.User{Email: "other@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	if err := users.Create(ctx, other); err != nil {
		t.Fatalf("Create other: %v", err)
	}

	gmToken, err := tokens.Issue(gm.ID)
	if err != nil {
		t.Fatalf("Issue(gm): %v", err)
	}

	playerToken, err := tokens.Issue(player.ID)
	if err != nil {
		t.Fatalf("Issue(player): %v", err)
	}

	otherToken, err := tokens.Issue(other.ID)
	if err != nil {
		t.Fatalf("Issue(other): %v", err)
	}

	character := &models.Character{Name: "Aria", UserID: player.ID}
	if err := characters.Create(ctx, character); err != nil {
		t.Fatalf("Create character: %v", err)
	}

	router := transport.NewRouter(
		transport.WithHandle(
			"GET /api/characters/{id}/journal", requireAuth(transport.ListJournalEntriesHandler(journalSvc)),
		),
		transport.WithHandle(
			"POST /api/characters/{id}/journal", requireAuth(transport.CreateJournalEntryHandler(journalSvc)),
		),
		transport.WithHandle(
			"GET /api/characters/{id}/journal/{entryId}", requireAuth(transport.GetJournalEntryHandler(journalSvc)),
		),
		transport.WithHandle(
			"PATCH /api/characters/{id}/journal/{entryId}",
			requireAuth(transport.UpdateJournalEntryHandler(journalSvc)),
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

func TestCreateJournalEntry_RequiresAuth(t *testing.T) {
	env := newJournalTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/journal", "",
		map[string]string{"content": "Dear diary"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no token, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateJournalEntry_OwnerCanWrite(t *testing.T) {
	env := newJournalTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/journal", env.playerToken,
		map[string]string{"content": "Dear diary"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created struct {
		Content string `json:"content"`
		GameDay int    `json:"game_day"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if created.Content != "Dear diary" {
		t.Fatalf("Content = %q, want %q", created.Content, "Dear diary")
	}
}

func TestCreateJournalEntry_OtherPlayerCannotWriteForForeignCharacter(t *testing.T) {
	env := newJournalTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/journal", env.otherToken,
		map[string]string{"content": "Not mine to write"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListJournalEntries_OtherPlayerHidden(t *testing.T) {
	env := newJournalTestEnv(t)

	env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/journal", env.playerToken,
		map[string]string{"content": "Secret entry"})

	rec := env.doJSON(t, http.MethodGet, "/api/characters/"+env.characterID+"/journal", env.otherToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListJournalEntries_GMCanReadAnyCharacter(t *testing.T) {
	env := newJournalTestEnv(t)

	env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/journal", env.playerToken,
		map[string]string{"content": "Entry one"})

	rec := env.doJSON(t, http.MethodGet, "/api/characters/"+env.characterID+"/journal", env.gmToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var list []struct{ Content string }
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(list))
	}
}

func TestGetJournalEntry_OtherPlayerHidden(t *testing.T) {
	env := newJournalTestEnv(t)

	createRec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/journal", env.playerToken,
		map[string]string{"content": "Secret entry"})

	var created struct{ ID string }
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding body: %v", err)
	}

	rec := env.doJSON(
		t, http.MethodGet, "/api/characters/"+env.characterID+"/journal/"+created.ID, env.otherToken, nil,
	)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateJournalEntry_OwnerCanEdit(t *testing.T) {
	env := newJournalTestEnv(t)

	createRec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/journal", env.playerToken,
		map[string]string{"content": "Original"})

	var created struct{ ID string }
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding body: %v", err)
	}

	rec := env.doJSON(
		t, http.MethodPatch, "/api/characters/"+env.characterID+"/journal/"+created.ID, env.playerToken,
		map[string]string{"content": "Revised"},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated struct{ Content string }
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if updated.Content != "Revised" {
		t.Fatalf("Content = %q, want %q", updated.Content, "Revised")
	}
}

func TestUpdateJournalEntry_OtherPlayerCannotEdit(t *testing.T) {
	env := newJournalTestEnv(t)

	createRec := env.doJSON(t, http.MethodPost, "/api/characters/"+env.characterID+"/journal", env.playerToken,
		map[string]string{"content": "Original"})

	var created struct{ ID string }
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding body: %v", err)
	}

	rec := env.doJSON(
		t, http.MethodPatch, "/api/characters/"+env.characterID+"/journal/"+created.ID, env.otherToken,
		map[string]string{"content": "Hijacked"},
	)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
