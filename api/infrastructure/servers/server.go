// Package servers wraps *http.Server with functional options and the
// graceful-shutdown lifecycle.
package servers

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Server wraps *http.Server with sane defaults (10 s read-header timeout).
type Server struct {
	srv *http.Server
}

// Option configures New via the functional-options pattern.
type Option func(*Server)

// WithAddr sets the listen address (default ":8080").
func WithAddr(addr string) Option {
	return func(s *Server) { s.srv.Addr = addr }
}

// WithHandler sets the root handler, typically a transport.Router.
func WithHandler(h http.Handler) Option {
	return func(s *Server) { s.srv.Handler = h }
}

// New builds a server with defaults applied, then options.
func New(opts ...Option) *Server {
	s := &Server{
		srv: &http.Server{
			Addr:              ":8080",
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Addr reports the configured listen address.
func (s *Server) Addr() string { return s.srv.Addr }

// ListenAndServe blocks until the server stops. A graceful shutdown returns
// nil, not http.ErrServerClosed.
func (s *Server) ListenAndServe() error {
	err := s.srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

// Shutdown drains in-flight requests until the context deadline
// (lifecycle.Shutdown phase).
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

// Handler reports the configured root handler.
func Handler(s *Server) http.Handler { return s.srv.Handler }

// ReadHeaderTimeout reports the configured read-header timeout.
func ReadHeaderTimeout(s *Server) time.Duration { return s.srv.ReadHeaderTimeout }
