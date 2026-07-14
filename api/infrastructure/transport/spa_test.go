package transport_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
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

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	if got := rec.Body.String(); got != "<html>shell</html>" {
		t.Fatalf("body = %q, want the SPA shell", got)
	}
}

func TestSPAHandler_ServesExistingFiles(t *testing.T) {
	rec := serveSPA(t, "/favicon.png")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	if got := rec.Body.String(); got != "png-bytes" {
		t.Fatalf("body = %q, want the asset content", got)
	}
}

func TestSPAHandler_ImmutableAssetsGetCacheHeader(t *testing.T) {
	rec := serveSPA(t, "/_app/immutable/entry/app.js")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q, want immutable caching", got)
	}
}

func TestSPAHandler_FallsBackToShellForClientRoutes(t *testing.T) {
	rec := serveSPA(t, "/characters/some-id")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	if got := rec.Body.String(); got != "<html>shell</html>" {
		t.Fatalf("body = %q, want the SPA shell", got)
	}
}

func TestSPAHandler_UnknownAPIPathsStay404(t *testing.T) {
	rec := serveSPA(t, "/api/does-not-exist")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
