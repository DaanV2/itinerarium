package transport_test

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DaanV2/itinerarium/api/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityHeaders_SetOnEveryResponse(t *testing.T) {
	router := transport.NewRouter(
		transport.WithMiddleware(transport.SecurityHeaders("", false)),
		transport.WithHandle("GET /api/health", transport.HealthHandler()),
	)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", http.NoBody))

	h := rec.Header()
	assert.Equal(t, "nosniff", h.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", h.Get("X-Frame-Options"))
	assert.Equal(t, "no-referrer", h.Get("Referrer-Policy"))
	assert.Equal(t, transport.DefaultCSP, h.Get("Content-Security-Policy"))
	assert.Empty(t, h.Get("Strict-Transport-Security"), "HSTS must be off for a plaintext request")
}

func TestSecurityHeaders_CustomCSP(t *testing.T) {
	const csp = "default-src 'none'"
	router := transport.NewRouter(
		transport.WithMiddleware(transport.SecurityHeaders(csp, false)),
		transport.WithHandle("GET /api/health", transport.HealthHandler()),
	)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", http.NoBody))

	assert.Equal(t, csp, rec.Header().Get("Content-Security-Policy"))
}

func TestSecurityHeaders_HSTSWhenEnabled(t *testing.T) {
	router := transport.NewRouter(
		transport.WithMiddleware(transport.SecurityHeaders("", true)),
		transport.WithHandle("GET /api/health", transport.HealthHandler()),
	)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", http.NoBody))

	assert.NotEmpty(t, rec.Header().Get("Strict-Transport-Security"))
}

func TestSecurityHeaders_HSTSOnTLSRequest(t *testing.T) {
	router := transport.NewRouter(
		transport.WithMiddleware(transport.SecurityHeaders("", false)),
		transport.WithHandle("GET /api/health", transport.HealthHandler()),
	)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", http.NoBody)
	req.TLS = &tls.ConnectionState{}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.NotEmpty(t, rec.Header().Get("Strict-Transport-Security"),
		"a TLS request should get HSTS even when the flag is off")
}

// bodyEchoHandler reads the whole request body and reports whether the read
// failed (which is how a MaxBytesReader surfaces an over-limit body).
func bodyEchoHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)

			return
		}

		w.WriteHeader(http.StatusOK)
	})
}

func TestMaxBytes_AllowsBodyUnderLimit(t *testing.T) {
	router := transport.NewRouter(
		transport.WithMiddleware(transport.MaxBytes(16)),
		transport.WithHandle("POST /echo", bodyEchoHandler()),
	)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/echo", strings.NewReader("small"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMaxBytes_RejectsBodyOverLimit(t *testing.T) {
	router := transport.NewRouter(
		transport.WithMiddleware(transport.MaxBytes(8)),
		transport.WithHandle("POST /echo", bodyEchoHandler()),
	)

	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, "/echo", strings.NewReader(strings.Repeat("x", 64)),
	)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code,
		"reading past the limit must fail")
}

func TestMaxBytes_ZeroLimitDisablesCap(t *testing.T) {
	router := transport.NewRouter(
		transport.WithMiddleware(transport.MaxBytes(0)),
		transport.WithHandle("POST /echo", bodyEchoHandler()),
	)

	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, "/echo", strings.NewReader(strings.Repeat("x", 4096)),
	)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}
