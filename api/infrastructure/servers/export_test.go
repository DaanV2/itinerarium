package servers

import (
	"net/http"
	"time"
)

// Test-only accessors for external tests (export_test.go convention).

func HandlerOf(s *Server) http.Handler { return s.srv.Handler }

func ReadHeaderTimeoutOf(s *Server) time.Duration { return s.srv.ReadHeaderTimeout }
