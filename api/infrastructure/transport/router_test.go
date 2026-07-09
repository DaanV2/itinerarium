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

func TestGroupMiddlewareWrapsOnlyGroupRoutes(t *testing.T) {
	var seen []string

	router := transport.NewRouter(
		transport.WithHandle("GET /public", transport.HealthHandler()),
		transport.WithGroup(recordMiddleware(&seen, "auth"),
			transport.WithHandle("GET /private", transport.HealthHandler()),
		),
	)

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/public", http.NoBody))

	if len(seen) != 0 {
		t.Fatalf("group middleware should not run for public route, saw %v", seen)
	}

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/private", http.NoBody))

	if len(seen) != 1 || seen[0] != "auth" {
		t.Fatalf("group middleware should wrap group route once, saw %v", seen)
	}
}

func TestNestedGroupsWrapOuterFirst(t *testing.T) {
	var order []string

	router := transport.NewRouter(
		transport.WithGroup(recordMiddleware(&order, "outer"),
			transport.WithGroup(recordMiddleware(&order, "inner"),
				transport.WithHandle("GET /deep", transport.HealthHandler()),
			),
		),
	)
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/deep", http.NoBody))

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
