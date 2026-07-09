// Package transport owns HTTP routing, handlers, and middleware. Handlers
// call services in application/ — business logic and permission checks never
// live here.
package transport

import (
	"net/http"
	"strings"
	"sync"
)

// Middleware wraps a handler; see Logging for an example.
type Middleware func(http.Handler) http.Handler

// route is a pattern/handler pair collected before the mux is built.
type route struct {
	pattern string
	handler http.Handler
}

// Router collects routes and middleware, then assembles an http.ServeMux
// lazily on first use. Because routes are held as data until then, a Router
// can also be mounted into another with WithSubRoute — a first-class
// subrouter, rather than a view over a shared mux.
type Router struct {
	routes     []route
	middleware []Middleware

	once    sync.Once
	handler http.Handler
}

// Option configures NewRouter via the functional-options pattern.
type Option func(*Router)

// NewRouter collects the routes and middleware described by opts. The mux is
// built on the first request (or when this router is mounted into another).
func NewRouter(opts ...Option) *Router {
	r := &Router{}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

// WithHandle registers a handler on a ServeMux pattern, e.g.
// "GET /api/health" or "POST /api/characters/{id}/journal".
func WithHandle(pattern string, handler http.Handler) Option {
	return func(r *Router) {
		r.routes = append(r.routes, route{pattern: pattern, handler: handler})
	}
}

// WithMiddleware appends router-wide middleware; it runs in registration order
// around every route (including unmatched ones).
func WithMiddleware(mw Middleware) Option {
	return func(r *Router) { r.middleware = append(r.middleware, mw) }
}

// WithSubRoute mounts sub's routes under prefix. Each route is rebased onto
// prefix and wrapped in sub's own middleware, so a subrouter is a
// self-contained group you build with NewRouter and mount here. A "/" route
// maps to the prefix itself (GET / under "/api/items" → GET /api/items); an
// empty prefix mounts sub's routes unchanged, which is how one middleware
// (e.g. auth) is shared across several prefixes at once.
func WithSubRoute(prefix string, sub *Router) Option {
	return func(r *Router) {
		for _, rt := range sub.routes {
			handler := rt.handler
			for i := len(sub.middleware) - 1; i >= 0; i-- {
				handler = sub.middleware[i](handler)
			}

			r.routes = append(r.routes, route{
				pattern: rebase(prefix, rt.pattern),
				handler: handler,
			})
		}
	}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.once.Do(r.build)
	r.handler.ServeHTTP(w, req)
}

// build assembles the mux from the collected routes and wraps it in the
// router-wide middleware. Called once, on the first request.
func (r *Router) build() {
	mux := http.NewServeMux()
	for _, rt := range r.routes {
		mux.Handle(rt.pattern, rt.handler)
	}

	handler := http.Handler(mux)
	for i := len(r.middleware) - 1; i >= 0; i-- {
		handler = r.middleware[i](handler)
	}

	r.handler = handler
}

// rebase rewrites a subrouter pattern to sit under prefix, preserving any
// leading HTTP method. A bare "/" path collapses onto the prefix so a
// collection root maps to the prefix itself.
func rebase(prefix, pattern string) string {
	method, path := splitPattern(pattern)
	if path == "/" {
		path = ""
	}

	joined := prefix + path
	if joined == "" {
		joined = "/"
	}

	if method == "" {
		return joined
	}

	return method + " " + joined
}

// splitPattern separates an optional leading method from the path of a
// ServeMux pattern ("GET /foo" → "GET", "/foo"; "/foo" → "", "/foo").
func splitPattern(pattern string) (method, path string) {
	if i := strings.IndexByte(pattern, ' '); i >= 0 {
		return pattern[:i], pattern[i+1:]
	}

	return "", pattern
}
