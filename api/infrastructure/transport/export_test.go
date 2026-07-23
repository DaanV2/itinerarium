package transport

import (
	"net/http"
	"time"
)

// ClientIP exposes the unexported client-IP resolver for testing.
func ClientIP(r *http.Request, trustProxy bool) string {
	return clientIP(r, trustProxy)
}

// NewTestThrottle builds a Throttle with an injectable clock so tests can drive
// lockout expiry deterministically instead of sleeping. Returns nil for a
// non-positive maxFailures, exactly like NewLoginThrottle.
func NewTestThrottle(maxFailures int, baseLockout time.Duration, now func() time.Time) *Throttle {
	t := NewLoginThrottle(maxFailures, baseLockout)
	if t != nil {
		t.now = now
	}

	return t
}
