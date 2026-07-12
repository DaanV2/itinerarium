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

type usersTestEnv struct {
	router      *transport.Router
	gmToken     string
	playerToken string
}

func newUsersTestEnv(t *testing.T) usersTestEnv {
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
	authSvc := application.NewAuthService(tokens, users)
	userSvc := application.NewUserService(users)
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

	gmToken, err := tokens.Issue(gm.ID)
	if err != nil {
		t.Fatalf("Issue(gm): %v", err)
	}

	playerToken, err := tokens.Issue(player.ID)
	if err != nil {
		t.Fatalf("Issue(player): %v", err)
	}

	router := transport.NewRouter(
		transport.WithHandle("GET /api/admin/users", requireAuth(transport.ListAccountsHandler(userSvc))),
		transport.WithHandle("POST /api/admin/users", requireAuth(transport.CreateAccountHandler(userSvc))),
		transport.WithHandle(
			"POST /api/admin/users/{id}/reset-password",
			requireAuth(transport.ResetPasswordHandler(userSvc)),
		),
	)

	return usersTestEnv{router: router, gmToken: gmToken, playerToken: playerToken}
}

func (e usersTestEnv) doJSON(t *testing.T, method, path, token string, payload any) *httptest.ResponseRecorder {
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

func TestCreateAccount_RequiresAuth(t *testing.T) {
	env := newUsersTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/admin/users", "", map[string]string{
		"email": "new@example.com", "role": "player",
	})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no token, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateAccount_RejectsPlayer(t *testing.T) {
	env := newUsersTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/admin/users", env.playerToken, map[string]string{
		"email": "new@example.com", "role": "player",
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a player caller, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateAccount_GMCreatesAccount(t *testing.T) {
	env := newUsersTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/admin/users", env.gmToken, map[string]string{
		"email": "new-player@example.com", "role": "player",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID                string `json:"id"`
		Email             string `json:"email"`
		Role              string `json:"role"`
		TemporaryPassword string `json:"temporary_password"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if created.Role != "player" {
		t.Fatalf("Role = %q, want player", created.Role)
	}
	if created.TemporaryPassword == "" {
		t.Fatal("expected a non-empty temporary password")
	}
}

func TestListAccounts_RejectsPlayer(t *testing.T) {
	env := newUsersTestEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/admin/users", env.playerToken, nil)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a player caller, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListAccounts_GM(t *testing.T) {
	env := newUsersTestEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/admin/users", env.gmToken, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var accounts []struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &accounts); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts (gm + player), got %d", len(accounts))
	}
}

func TestResetPassword_RejectsPlayer(t *testing.T) {
	env := newUsersTestEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/admin/users", env.gmToken, nil)

	var accounts []struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &accounts); err != nil {
		t.Fatalf("decoding body: %v", err)
	}

	targetID := accounts[0].ID

	resetRec := env.doJSON(t, http.MethodPost, "/api/admin/users/"+targetID+"/reset-password", env.playerToken, nil)
	if resetRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a player caller, got %d: %s", resetRec.Code, resetRec.Body.String())
	}
}

func TestResetPassword_GM(t *testing.T) {
	env := newUsersTestEnv(t)

	listRec := env.doJSON(t, http.MethodGet, "/api/admin/users", env.gmToken, nil)

	var accounts []struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &accounts); err != nil {
		t.Fatalf("decoding body: %v", err)
	}

	var targetID string
	for _, a := range accounts {
		if a.Email == "player@example.com" {
			targetID = a.ID
		}
	}
	if targetID == "" {
		t.Fatal("could not find player account in list")
	}

	rec := env.doJSON(t, http.MethodPost, "/api/admin/users/"+targetID+"/reset-password", env.gmToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		TemporaryPassword string `json:"temporary_password"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if body.TemporaryPassword == "" {
		t.Fatal("expected a non-empty temporary password")
	}
}

func TestResetPassword_UnknownAccount(t *testing.T) {
	env := newUsersTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/admin/users/does-not-exist/reset-password", env.gmToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
