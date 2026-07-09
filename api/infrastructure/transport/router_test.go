package transport_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
)

func TestHealthRoute(t *testing.T) {
	router := transport.NewRouter(
		transport.WithHandle("GET /api/health", transport.HealthHandler()),
	)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", http.NoBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if got := rec.Body.String(); got != `{"status":"ok"}` {
		t.Fatalf("unexpected body: %s", got)
	}
}

func TestUnknownRouteIs404(t *testing.T) {
	router := transport.NewRouter()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/nope", http.NoBody))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
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

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", path, rec.Code)
		}
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

	if len(seen) != 0 {
		t.Fatalf("sub middleware should not run for a route outside the subrouter, saw %v", seen)
	}

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/private", http.NoBody))

	if len(seen) != 1 || seen[0] != "auth" {
		t.Fatalf("sub middleware should wrap the sub route once, saw %v", seen)
	}
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

	if len(order) != 2 || order[0] != "outer" || order[1] != "inner" {
		t.Fatalf("expected outer then inner, got %v", order)
	}
}

func TestMiddlewareRunsInRegistrationOrder(t *testing.T) {
	var order []string

	router := transport.NewRouter(
		transport.WithMiddleware(recordMiddleware(&order, "first")),
		transport.WithMiddleware(recordMiddleware(&order, "second")),
		transport.WithHandle("GET /", transport.HealthHandler()),
	)
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody))

	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Fatalf("unexpected middleware order: %v", order)
	}
}
