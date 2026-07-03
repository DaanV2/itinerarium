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
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
)

func newSetupRouter(t *testing.T) *transport.Router {
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
	svc := application.NewSetupService(repositories.NewUsers(db), tokens)

	return transport.NewRouter(
		transport.WithHandle("GET /api/setup", transport.SetupStatusHandler(svc)),
		transport.WithHandle("POST /api/setup", transport.CreateInitialGMHandler(svc)),
	)
}

func TestSetupStatus_NeedsSetupInitially(t *testing.T) {
	router := newSetupRouter(t)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/setup", http.NoBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body struct {
		NeedsSetup bool `json:"needs_setup"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if !body.NeedsSetup {
		t.Fatal("expected needs_setup=true on a fresh installation")
	}
}

func TestCreateInitialGM_SucceedsOnce(t *testing.T) {
	router := newSetupRouter(t)

	body, err := json.Marshal(map[string]string{"email": "gm@example.com", "password": "hunter22hunter"})
	if err != nil {
		t.Fatalf("marshalling request: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/setup", bytes.NewReader(body))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID          string `json:"id"`
		Email       string `json:"email"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if created.AccessToken == "" {
		t.Fatal("expected a non-empty access token")
	}
}

func TestCreateInitialGM_RefusesSecondCall(t *testing.T) {
	router := newSetupRouter(t)

	body, err := json.Marshal(map[string]string{"email": "gm@example.com", "password": "hunter22hunter"})
	if err != nil {
		t.Fatalf("marshalling request: %v", err)
	}

	first := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/setup", bytes.NewReader(body))
	router.ServeHTTP(httptest.NewRecorder(), first)

	second := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/setup", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, second)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 on the second call, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateInitialGM_RejectsInvalidInput(t *testing.T) {
	router := newSetupRouter(t)

	body, err := json.Marshal(map[string]string{"email": "not-an-email", "password": "short"})
	if err != nil {
		t.Fatalf("marshalling request: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/setup", bytes.NewReader(body))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
