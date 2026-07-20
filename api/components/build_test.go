package components_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DaanV2/itinerarium/api/components"
	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildServerWiresWorkingServer is the composition-root smoke test: the
// M7–M8 refactors move code through BuildServer's wiring, so a wiring
// regression should fail a test rather than only surface at runtime. It builds
// the whole server against an in-memory database and drives one request end to
// end through the assembled router.
func TestBuildServerWiresWorkingServer(t *testing.T) {
	t.Setenv("DATABASE_TYPE", "memory")
	t.Setenv("AUTH_KEYS_PATH", t.TempDir())

	sc, err := components.BuildServer(t.Context())
	require.NoError(t, err)
	require.NotNil(t, sc)
	t.Cleanup(func() { _ = sc.Shutdown(t.Context()) })

	require.NotNil(t, sc.DB)
	require.NotNil(t, sc.Repositories)
	require.NotNil(t, sc.Services)
	require.NotNil(t, sc.Server)
	assert.NotEmpty(t, sc.Server.Addr())

	// The database is live and migrated: a fresh install still needs setup.
	needsSetup, err := sc.Services.Setup.NeedsSetup(t.Context())
	require.NoError(t, err)
	assert.True(t, needsSetup)

	// The router assembles and serves: the setup-status route responds through
	// the same wiring BuildServer used.
	router := components.CreateRouter(sc.Services, log.Default())
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/setup", http.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "needs_setup")
}
