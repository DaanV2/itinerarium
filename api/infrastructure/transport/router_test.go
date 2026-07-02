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

func TestMiddlewareRunsInRegistrationOrder(t *testing.T) {
	var order []string

	tag := func(name string) transport.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}

	router := transport.NewRouter(
		transport.WithMiddleware(tag("first")),
		transport.WithMiddleware(tag("second")),
		transport.WithHandle("GET /", transport.HealthHandler()),
	)
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody))

	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Fatalf("unexpected middleware order: %v", order)
	}
}
