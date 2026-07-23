package component

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DaanV2/itinerarium/api/components"
	"github.com/DaanV2/itinerarium/api/transport/server"
	"github.com/stretchr/testify/require"
)

// Harness is a fully assembled server under test. It boots the real
// composition root against an in-memory database and serves it over a real HTTP
// listener, so tests exercise the same wiring production uses.
type Harness struct {
	T          *testing.T
	Components *components.ServerComponents
	HTTP       *httptest.Server
	Client     *http.Client
}

// Response is a fully-read HTTP response: Do drains and closes the body, so
// tests never manage its lifetime. Decode the payload with DecodeJSON.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte

	t *testing.T
}

// New builds an isolated server for one test: an in-memory database, freshly
// generated signing keys in a temp directory, and a real HTTP listener. Every
// resource is torn down automatically when the test finishes.
func New(t *testing.T) *Harness {
	t.Helper()

	// Each harness gets its own throwaway database and key material so tests
	// stay independent. BuildServer reads both through their config sets.
	t.Setenv("DATABASE_TYPE", "memory")
	t.Setenv("AUTH_KEYS_PATH", t.TempDir())

	sc, err := components.BuildServer(t.Context())
	require.NoError(t, err, "building server")

	srv := httptest.NewServer(server.Handler(sc.Server))

	t.Cleanup(func() {
		srv.Close()
		_ = sc.Shutdown(context.Background())
	})

	return &Harness{
		T:          t,
		Components: sc,
		HTTP:       srv,
		Client:     srv.Client(),
	}
}

// URL resolves a server-relative path (e.g. "/api/setup") to an absolute URL on
// the test listener.
func (h *Harness) URL(path string) string {
	return h.HTTP.URL + path
}

// Do issues an HTTP request against the test server and returns the fully-read
// response. A non-nil body is JSON-encoded; a non-empty token is sent as a
// bearer token.
func (h *Harness) Do(method, path, token string, body any) *Response {
	h.T.Helper()

	var reader io.Reader = http.NoBody
	if body != nil {
		encoded, err := json.Marshal(body)
		require.NoError(h.T, err, "encoding request body")

		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(h.T.Context(), method, h.URL(path), reader)
	require.NoError(h.T, err, "building request")

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := h.Client.Do(req)
	require.NoError(h.T, err, "issuing request")
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	require.NoError(h.T, err, "reading response body")

	return &Response{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       data,
		t:          h.T,
	}
}

// DecodeJSON unmarshals the response body into v, failing the test on a decode
// error.
func (r *Response) DecodeJSON(v any) {
	r.t.Helper()

	require.NoError(r.t, json.Unmarshal(r.Body, v), "decoding response body: %s", r.Body)
}

// String renders the response body as text, handy in assertion messages.
func (r *Response) String() string {
	return string(r.Body)
}

// CreateGM runs the first-run setup flow and returns the new game master's
// access token, failing the test if setup does not succeed.
func (h *Harness) CreateGM(email, password string) string {
	h.T.Helper()

	resp := h.Do(http.MethodPost, "/api/setup", "", map[string]string{
		"email":    email,
		"password": password,
	})
	require.Equal(h.T, http.StatusCreated, resp.StatusCode, "setup should succeed: %s", resp)

	var created struct {
		AccessToken string `json:"access_token"`
	}
	resp.DecodeJSON(&created)
	require.NotEmpty(h.T, created.AccessToken, "setup should return an access token")

	return created.AccessToken
}
