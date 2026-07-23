package transport

import (
	"net/http"
)

// DefaultCSP is a minimal Content-Security-Policy for the embedded SvelteKit
// SPA. Everything is same-origin, so default-src 'self' covers the app; the
// SvelteKit bootstrap and component styles are injected inline, so scripts and
// styles allow 'unsafe-inline' (adapter-static emits no nonce to pin them to).
// Framing and plugins are denied outright. Operators serving a tightened build
// can override the whole policy via security.csp.
const DefaultCSP = "default-src 'self'; base-uri 'self'; object-src 'none'; " +
	"frame-ancestors 'none'; img-src 'self' data:; font-src 'self' data:; " +
	"style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; " +
	"connect-src 'self'; form-action 'self'"

// hstsValue asks browsers to pin HTTPS for two years, including subdomains.
const hstsValue = "max-age=63072000; includeSubDomains"

// SecurityHeaders adds a standard set of hardening response headers to every
// request: nosniff, deny framing, a referrer policy, and the given CSP. When
// hsts is true (operator serves behind TLS) or the request itself arrived over
// TLS, it also sends Strict-Transport-Security.
func SecurityHeaders(csp string, hsts bool) Middleware {
	if csp == "" {
		csp = DefaultCSP
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Content-Security-Policy", csp)

			if hsts || r.TLS != nil {
				h.Set("Strict-Transport-Security", hstsValue)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// MaxBytes caps every request body at limit bytes so a single request cannot
// exhaust server memory. Reads past the limit fail, which handlers surface as
// a 400 (or 413) when decoding. A non-positive limit disables the cap.
func MaxBytes(limit int64) Middleware {
	return func(next http.Handler) http.Handler {
		if limit <= 0 {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}

			next.ServeHTTP(w, r)
		})
	}
}
