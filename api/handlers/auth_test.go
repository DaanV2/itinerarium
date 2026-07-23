package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/handlers"
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/DaanV2/itinerarium/api/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLoginTestRouter(t *testing.T, email, password string, throttle *transport.Throttle) *transport.Router {
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
		transport.WithHandle("POST /api/login", handlers.LoginHandler(authSvc, throttle, false)),
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
	router := newLoginTestRouter(t, "player@example.com", "hunter22hunter", nil)

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
	router := newLoginTestRouter(t, "player@example.com", "hunter22hunter", nil)

	rec := doLogin(t, router, "player@example.com", "wrong-password")
	require.Equal(t, http.StatusUnauthorized, rec.Code, "body: %s", rec.Body.String())
}

func TestLogin_RejectsUnknownEmail(t *testing.T) {
	router := newLoginTestRouter(t, "player@example.com", "hunter22hunter", nil)

	rec := doLogin(t, router, "nobody@example.com", "hunter22hunter")
	require.Equal(t, http.StatusUnauthorized, rec.Code, "body: %s", rec.Body.String())
}

func TestLogin_RejectsInvalidBody(t *testing.T) {
	router := newLoginTestRouter(t, "player@example.com", "hunter22hunter", nil)

	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, "/api/login", bytes.NewBufferString("not-json"),
	)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
}

func TestLogin_ThrottlesAfterRepeatedFailures(t *testing.T) {
	now := time.Now()
	throttle := transport.NewLoginThrottle(3, time.Minute).WithClock(func() time.Time { return now })
	router := newLoginTestRouter(t, "player@example.com", "hunter22hunter", throttle)

	for i := range 3 {
		rec := doLogin(t, router, "player@example.com", "wrong-password")
		require.Equalf(t, http.StatusUnauthorized, rec.Code, "attempt %d should be a plain 401", i+1)
	}

	// The next attempt is locked out before credentials are even checked.
	rec := doLogin(t, router, "player@example.com", "wrong-password")
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"), "a 429 must tell the client when to retry")

	// Even the correct password is refused while the account is locked.
	rec = doLogin(t, router, "player@example.com", "hunter22hunter")
	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "a lockout ignores whether the credentials are right")
}

func TestLogin_ThrottleRecoversAfterLockout(t *testing.T) {
	now := time.Now()
	throttle := transport.NewLoginThrottle(3, time.Minute).WithClock(func() time.Time { return now })
	router := newLoginTestRouter(t, "player@example.com", "hunter22hunter", throttle)

	for range 3 {
		doLogin(t, router, "player@example.com", "wrong-password")
	}
	require.Equal(t, http.StatusTooManyRequests, doLogin(t, router, "player@example.com", "hunter22hunter").Code)

	now = now.Add(2 * time.Minute) // past the base lockout
	rec := doLogin(t, router, "player@example.com", "hunter22hunter")
	assert.Equal(t, http.StatusOK, rec.Code, "the lockout should expire; body: %s", rec.Body.String())
}

func TestLogin_ThrottlesPerIPAcrossAccounts(t *testing.T) {
	now := time.Now()
	throttle := transport.NewLoginThrottle(3, time.Minute).WithClock(func() time.Time { return now })
	router := newLoginTestRouter(t, "player@example.com", "hunter22hunter", throttle)

	// Three failures spread across different accounts: no single account reaches
	// the threshold, but the shared client IP does.
	for _, email := range []string{"a@example.com", "b@example.com", "c@example.com"} {
		require.Equal(t, http.StatusUnauthorized, doLogin(t, router, email, "wrong-password").Code)
	}

	// A fourth attempt from the same IP is locked out regardless of account —
	// even the correct credentials for the real account.
	rec := doLogin(t, router, "player@example.com", "hunter22hunter")
	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "the per-IP lockout applies across accounts")
}

func TestLogin_NilThrottleNeverLocks(t *testing.T) {
	router := newLoginTestRouter(t, "player@example.com", "hunter22hunter", nil)

	// Far more failures than any threshold — a disabled limiter must keep
	// returning plain 401s, never a 429.
	for range 20 {
		rec := doLogin(t, router, "player@example.com", "wrong-password")
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	}
}
