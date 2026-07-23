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
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/DaanV2/itinerarium/api/transport"
	"github.com/stretchr/testify/require"
)

func newSetupRouter(t *testing.T) *transport.Router {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err, "persistence.New")
	require.NoError(t, db.Migrate(), "Migrate")

	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(t.TempDir()))
	require.NoError(t, err, "NewKeyStore")

	tokens := authentication.NewTokenService(keys, repositories.NewRevokedTokens(db))
	svc := application.NewSetupService(repositories.NewUsers(db), tokens)

	return transport.NewRouter(
		transport.WithHandle("GET /api/setup", handlers.SetupStatusHandler(svc)),
		transport.WithHandle("POST /api/setup", handlers.CreateInitialGMHandler(svc)),
	)
}

func TestSetupStatus_NeedsSetupInitially(t *testing.T) {
	router := newSetupRouter(t)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/setup", http.NoBody))

	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		NeedsSetup bool `json:"needs_setup"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "decoding body")
	require.True(t, body.NeedsSetup, "expected needs_setup=true on a fresh installation")
}

func TestCreateInitialGM_SucceedsOnce(t *testing.T) {
	router := newSetupRouter(t)

	body, err := json.Marshal(map[string]string{"email": "gm@example.com", "password": "hunter22hunter"})
	require.NoError(t, err, "marshalling request")

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/setup", bytes.NewReader(body))
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	var created struct {
		ID          string `json:"id"`
		Email       string `json:"email"`
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created), "decoding body")
	require.NotEmpty(t, created.AccessToken, "expected a non-empty access token")
}

func TestCreateInitialGM_RefusesSecondCall(t *testing.T) {
	router := newSetupRouter(t)

	body, err := json.Marshal(map[string]string{"email": "gm@example.com", "password": "hunter22hunter"})
	require.NoError(t, err, "marshalling request")

	first := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/setup", bytes.NewReader(body))
	router.ServeHTTP(httptest.NewRecorder(), first)

	second := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/setup", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, second)

	require.Equal(t, http.StatusConflict, rec.Code, "second call body: %s", rec.Body.String())
}

func TestCreateInitialGM_RejectsInvalidInput(t *testing.T) {
	router := newSetupRouter(t)

	body, err := json.Marshal(map[string]string{"email": "not-an-email", "password": "short"})
	require.NoError(t, err, "marshalling request")

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/setup", bytes.NewReader(body))
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
}
