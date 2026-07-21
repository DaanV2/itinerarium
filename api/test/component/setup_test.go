package component_test

import (
	"net/http"
	"testing"

	"github.com/DaanV2/itinerarium/api/test/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetupFlow drives the first-run setup end to end over real HTTP: a fresh
// install reports that it needs setup, the setup call creates the initial GM
// and returns a token that authenticates against a protected route, and a
// second setup call is refused.
func TestSetupFlow(t *testing.T) {
	h := component.New(t)

	// A fresh install advertises that it needs setup.
	resp := h.Do(http.MethodGet, "/api/setup", "", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var status struct {
		NeedsSetup bool `json:"needs_setup"`
	}
	resp.DecodeJSON(&status)
	assert.True(t, status.NeedsSetup, "a fresh install should need setup")

	// Creating the initial GM succeeds and yields an access token...
	token := h.CreateGM("gm@example.com", "hunter22hunter")

	// ...that authenticates against a protected route.
	authed := h.Do(http.MethodGet, "/api/characters", token, nil)
	assert.Equal(t, http.StatusOK, authed.StatusCode, "the GM token should reach a protected route")

	// The same route without a token is rejected: auth is actually enforced.
	anon := h.Do(http.MethodGet, "/api/characters", "", nil)
	assert.Equal(t, http.StatusUnauthorized, anon.StatusCode, "an unauthenticated request should be rejected")

	// A second setup attempt is refused: setup runs exactly once.
	second := h.Do(http.MethodPost, "/api/setup", "", map[string]string{
		"email":    "other@example.com",
		"password": "hunter22hunter",
	})
	assert.Equal(t, http.StatusConflict, second.StatusCode, "setup should run only once")
}
