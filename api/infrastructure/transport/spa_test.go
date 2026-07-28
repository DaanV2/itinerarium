package transport_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/DaanV2/itinerarium/api/transport"
	"github.com/stretchr/testify/require"
)

func spaAssets() fstest.MapFS {
	return fstest.MapFS{
		"index.html":                  {Data: []byte("<html>shell</html>")},
		"favicon.png":                 {Data: []byte("png-bytes")},
		"_app/immutable/entry/app.js": {Data: []byte("console.log('app')")},
	}
}

func serveSPA(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()

	handler := transport.SPAHandler(spaAssets())
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

func TestSPAHandler_ServesShellAtRoot(t *testing.T) {
	rec := serveSPA(t, "/")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "<html>shell</html>", rec.Body.String(), "want the SPA shell")
}

func TestSPAHandler_ServesExistingFiles(t *testing.T) {
	rec := serveSPA(t, "/favicon.png")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "png-bytes", rec.Body.String(), "want the asset content")
}

func TestSPAHandler_ImmutableAssetsGetCacheHeader(t *testing.T) {
	rec := serveSPA(t, "/_app/immutable/entry/app.js")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
}

func TestSPAHandler_FallsBackToShellForClientRoutes(t *testing.T) {
	rec := serveSPA(t, "/characters/some-id")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "<html>shell</html>", rec.Body.String(), "want the SPA shell")
}

func TestSPAHandler_UnknownAPIPathsStay404(t *testing.T) {
	rec := serveSPA(t, "/api/does-not-exist")

	require.Equal(t, http.StatusNotFound, rec.Code)
}
