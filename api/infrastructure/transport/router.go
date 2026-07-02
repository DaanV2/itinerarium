// Package transport owns HTTP routing, handlers, and middleware. Handlers
// call services in application/ — business logic and permission checks never
// live here.
package transport

import "net/http"

// Middleware wraps a handler; see Logging for an example.
type Middleware func(http.Handler) http.Handler

// Router is an http.ServeMux assembled through functional options.
type Router struct {
	mux        *http.ServeMux
	middleware []Middleware
	handler    http.Handler
}

// Option configures NewRouter via the functional-options pattern.
type Option func(*Router)

// WithHandle registers a handler on a ServeMux pattern, e.g.
// "GET /api/health" or "POST /api/characters/{id}/journal".
func WithHandle(pattern string, handler http.Handler) Option {
	return func(r *Router) { r.mux.Handle(pattern, handler) }
}

// WithMiddleware appends middleware; it runs in registration order around
// every route.
func WithMiddleware(mw Middleware) Option {
	return func(r *Router) { r.middleware = append(r.middleware, mw) }
}

// NewRouter builds the router and bakes the middleware chain.
func NewRouter(opts ...Option) *Router {
	r := &Router{mux: http.NewServeMux()}
	for _, opt := range opts {
		opt(r)
	}

	r.handler = r.mux
	for i := len(r.middleware) - 1; i >= 0; i-- {
		r.handler = r.middleware[i](r.handler)
	}

	return r
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.handler.ServeHTTP(w, req)
}
