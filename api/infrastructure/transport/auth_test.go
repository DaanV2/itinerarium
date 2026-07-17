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
	"github.com/stretchr/testify/require"
)

func newLoginTestRouter(t *testing.T, email, password string) *transport.Router {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err, "persistence.New")
	require.NoError(t, db.Migrate(), "Migrate")

	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(t.TempDir()))
	require.NoError(t, err, "NewKeyStore")

	tokens := authentication.NewTokenService(keys, repositories.NewRevokedTokens(db))
	users := repositories.NewUsers(db)
	authSvc := application.NewAuthService(tokens, users)

	hash, err := authentication.HashPassword(password)
	require.NoError(t, err, "HashPassword")

	user := &models.User{Email: email, PasswordHash: hash, Role: models.RolePlayer}
	require.NoError(t, users.Create(t.Context(), user), "Create")

	return transport.NewRouter(
		transport.WithHandle("POST /api/login", transport.LoginHandler(authSvc)),
	)
}

func doLogin(t *testing.T, router *transport.Router, email, password string) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(map[string]string{"email": email, "password": password})
	require.NoError(t, err, "encoding request")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/login", &body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec
}

func TestLogin_Succeeds(t *testing.T) {
	router := newLoginTestRouter(t, "player@example.com", "hunter22hunter")

	rec := doLogin(t, router, "player@example.com", "hunter22hunter")
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var body struct {
		ID          string `json:"id"`
		Email       string `json:"email"`
		Role        string `json:"role"`
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "decoding body")
	require.Equal(t, "player@example.com", body.Email)
	require.Equal(t, "player", body.Role)
	require.NotEmpty(t, body.AccessToken, "expected a non-empty access token")
}

func TestLogin_RejectsWrongPassword(t *testing.T) {
	router := newLoginTestRouter(t, "player@example.com", "hunter22hunter")

	rec := doLogin(t, router, "player@example.com", "wrong-password")
	require.Equal(t, http.StatusUnauthorized, rec.Code, "body: %s", rec.Body.String())
}

func TestLogin_RejectsUnknownEmail(t *testing.T) {
	router := newLoginTestRouter(t, "player@example.com", "hunter22hunter")

	rec := doLogin(t, router, "nobody@example.com", "hunter22hunter")
	require.Equal(t, http.StatusUnauthorized, rec.Code, "body: %s", rec.Body.String())
}

func TestLogin_RejectsInvalidBody(t *testing.T) {
	router := newLoginTestRouter(t, "player@example.com", "hunter22hunter")

	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, "/api/login", bytes.NewBufferString("not-json"),
	)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
}
