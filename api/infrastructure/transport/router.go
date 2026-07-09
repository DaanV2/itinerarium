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
	groupMW    []Middleware
	handler    http.Handler
}

// Option configures NewRouter via the functional-options pattern.
type Option func(*Router)

// WithHandle registers a handler on a ServeMux pattern, e.g.
// "GET /api/health" or "POST /api/characters/{id}/journal". Inside a
// WithGroup, the group's middleware wraps the handler.
func WithHandle(pattern string, handler http.Handler) Option {
	return func(r *Router) {
		for i := len(r.groupMW) - 1; i >= 0; i-- {
			handler = r.groupMW[i](handler)
		}

		r.mux.Handle(pattern, handler)
	}
}

// WithMiddleware appends router-wide middleware; it runs in registration order
// around every route.
func WithMiddleware(mw Middleware) Option {
	return func(r *Router) { r.middleware = append(r.middleware, mw) }
}

// WithGroup registers a set of routes that share one extra middleware — a
// subrouter. The middleware wraps only the handlers declared inside the group
// (after the router-wide middleware), so cross-cutting concerns like
// authentication are declared once instead of per route. Groups nest.
func WithGroup(mw Middleware, opts ...Option) Option {
	return func(r *Router) {
		group := &Router{
			mux:     r.mux,
			groupMW: append(append([]Middleware{}, r.groupMW...), mw),
		}
		for _, opt := range opts {
			opt(group)
		}
	}
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
