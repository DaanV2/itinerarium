package transport_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
	"github.com/stretchr/testify/require"
)

func TestHealthRoute(t *testing.T) {
	router := transport.NewRouter(
		transport.WithHandle("GET /api/health", transport.HealthHandler()),
	)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", http.NoBody))

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

func TestUnknownRouteIs404(t *testing.T) {
	router := transport.NewRouter()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/nope", http.NoBody))

	require.Equal(t, http.StatusNotFound, rec.Code)
}

// recordMiddleware returns middleware that appends name to order when it runs,
// so tests can assert which handlers a middleware wrapped and in what order.
func recordMiddleware(order *[]string, name string) transport.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*order = append(*order, name)
			next.ServeHTTP(w, r)
		})
	}
}

func TestSubRouteMountsRoutesUnderPrefix(t *testing.T) {
	sub := transport.NewRouter(
		transport.WithHandle("GET /", transport.HealthHandler()),     // collapses to the prefix
		transport.WithHandle("GET /ping", transport.HealthHandler()), // nests under the prefix
	)

	router := transport.NewRouter(
		transport.WithSubRoute("/api/items", sub),
	)

	for _, path := range []string{"/api/items", "/api/items/ping"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody))

		require.Equal(t, http.StatusOK, rec.Code, "path %s", path)
	}
}

func TestSubRouteAppliesItsOwnMiddleware(t *testing.T) {
	var seen []string

	sub := transport.NewRouter(
		transport.WithMiddleware(recordMiddleware(&seen, "auth")),
		transport.WithHandle("GET /private", transport.HealthHandler()),
	)

	router := transport.NewRouter(
		transport.WithHandle("GET /public", transport.HealthHandler()),
		transport.WithSubRoute("/api", sub),
	)

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/public", http.NoBody))

	require.Empty(t, seen, "sub middleware should not run for a route outside the subrouter")

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/private", http.NoBody))

	require.Equal(t, []string{"auth"}, seen, "sub middleware should wrap the sub route once")
}

func TestNestedSubRoutesWrapOuterFirst(t *testing.T) {
	var order []string

	inner := transport.NewRouter(
		transport.WithMiddleware(recordMiddleware(&order, "inner")),
		transport.WithHandle("GET /deep", transport.HealthHandler()),
	)
	outer := transport.NewRouter(
		transport.WithMiddleware(recordMiddleware(&order, "outer")),
		transport.WithSubRoute("/in", inner),
	)
	router := transport.NewRouter(
		transport.WithSubRoute("/api", outer),
	)
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/in/deep", http.NoBody))

	require.Equal(t, []string{"outer", "inner"}, order)
}

func TestMiddlewareRunsInRegistrationOrder(t *testing.T) {
	var order []string

	router := transport.NewRouter(
		transport.WithMiddleware(recordMiddleware(&order, "first")),
		transport.WithMiddleware(recordMiddleware(&order, "second")),
		transport.WithHandle("GET /", transport.HealthHandler()),
	)
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody))

	require.Equal(t, []string{"first", "second"}, order)
}
