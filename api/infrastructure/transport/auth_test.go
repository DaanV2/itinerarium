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

func newLoginTestRouter(t *testing.T, email, password string) *transport.Router {
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

	hash, err := authentication.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	user := &models.User{Email: email, PasswordHash: hash, Role: models.RolePlayer}
	if err := users.Create(t.Context(), user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	return transport.NewRouter(
		transport.WithHandle("POST /api/login", transport.LoginHandler(authSvc)),
	)
}

func doLogin(t *testing.T, router *transport.Router, email, password string) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(map[string]string{"email": email, "password": password}); err != nil {
		t.Fatalf("encoding request: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/login", &body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec
}

func TestLogin_Succeeds(t *testing.T) {
	router := newLoginTestRouter(t, "player@example.com", "hunter22hunter")

	rec := doLogin(t, router, "player@example.com", "hunter22hunter")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		ID          string `json:"id"`
		Email       string `json:"email"`
		Role        string `json:"role"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if body.Email != "player@example.com" {
		t.Fatalf("Email = %q, want player@example.com", body.Email)
	}
	if body.Role != "player" {
		t.Fatalf("Role = %q, want player", body.Role)
	}
	if body.AccessToken == "" {
		t.Fatal("expected a non-empty access token")
	}
}

func TestLogin_RejectsWrongPassword(t *testing.T) {
	router := newLoginTestRouter(t, "player@example.com", "hunter22hunter")

	rec := doLogin(t, router, "player@example.com", "wrong-password")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogin_RejectsUnknownEmail(t *testing.T) {
	router := newLoginTestRouter(t, "player@example.com", "hunter22hunter")

	rec := doLogin(t, router, "nobody@example.com", "hunter22hunter")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogin_RejectsInvalidBody(t *testing.T) {
	router := newLoginTestRouter(t, "player@example.com", "hunter22hunter")

	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, "/api/login", bytes.NewBufferString("not-json"),
	)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
