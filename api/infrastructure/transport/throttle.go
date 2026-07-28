package transport

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
)

// maxBackoffMultiplier caps the exponential lockout at 32× the base duration, so
// a sustained attack tops out at a bounded lockout instead of growing forever.
const maxBackoffMultiplier = 32

// throttleSoftCap bounds the number of tracked keys. Distinct client IPs are the
// only source of unbounded growth; once the map exceeds this, expired entries
// are swept on the next write. Kept high so a real deployment never sweeps.
const throttleSoftCap = 10000

// attemptState tracks failed attempts and the current lockout for one key.
type attemptState struct {
	failures    int
	lockedUntil time.Time
	lastSeen    time.Time
}

// Throttle is an in-memory, keyed attempt limiter with exponential lockout
// (roadmap M10). It guards credential-guessing surfaces (login) and abuse
// surfaces (password reset) without an external dependency. A nil *Throttle is a
// valid, always-allow limiter, so a caller disables the feature by passing nil
// instead of special-casing every call site.
type Throttle struct {
	mu          sync.Mutex
	states      map[string]*attemptState
	maxFailures int
	baseLockout time.Duration
	now         func() time.Time
}

// NewLoginThrottle builds a Throttle that locks a key after maxFailures failed
// attempts for baseLockout (doubling per further failure up to
// maxBackoffMultiplier). It returns nil — the always-allow limiter — when
// maxFailures is not positive, so the feature is disabled purely by config.
func NewLoginThrottle(maxFailures int, baseLockout time.Duration) *Throttle {
	if maxFailures <= 0 {
		return nil
	}

	return &Throttle{
		states:      make(map[string]*attemptState),
		maxFailures: maxFailures,
		baseLockout: baseLockout,
		now:         time.Now,
	}
}

// WithClock overrides the throttle's time source so callers (chiefly tests) can
// drive lockout expiry deterministically instead of sleeping. A nil Throttle is
// returned unchanged, mirroring NewLoginThrottle's disabled-limiter contract.
func (t *Throttle) WithClock(now func() time.Time) *Throttle {
	if t != nil {
		t.now = now
	}

	return t
}

// Allowed reports whether key may attempt now. When locked it returns false and
// the remaining lockout, for a Retry-After header. A nil Throttle always allows.
func (t *Throttle) Allowed(key string) (bool, time.Duration) {
	if t == nil {
		return true, 0
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	st, ok := t.states[key]
	if !ok {
		return true, 0
	}

	now := t.now()
	if now.Before(st.lockedUntil) {
		return false, st.lockedUntil.Sub(now)
	}

	return true, 0
}

// Penalize records a bad attempt (a failed login, or a chargeable reset) for key
// and locks it once failures reach the configured threshold. State that has sat
// idle past the retention window is treated as fresh, so failures decay over
// time. A nil Throttle does nothing.
func (t *Throttle) Penalize(key string) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.now()

	st := t.states[key]
	if st == nil || t.isExpired(st, now) {
		st = &attemptState{}
		t.states[key] = st
	}

	st.failures++
	st.lastSeen = now
	if st.failures >= t.maxFailures {
		st.lockedUntil = now.Add(t.backoff(st.failures))
	}

	t.sweepLocked(now)
}

// Reset clears key after a good attempt. A nil Throttle does nothing.
func (t *Throttle) Reset(key string) {
	if t == nil {
		return
	}

	t.mu.Lock()
	delete(t.states, key)
	t.mu.Unlock()
}

// backoff returns the lockout for the given failure count: baseLockout doubled
// once per failure beyond the threshold, capped at maxBackoffMultiplier.
func (t *Throttle) backoff(failures int) time.Duration {
	excess := failures - t.maxFailures // 0 on the first lock

	mult := maxBackoffMultiplier
	if excess < 5 { // 2^5 == maxBackoffMultiplier; beyond that we clamp
		mult = 1 << excess
	}

	return t.baseLockout * time.Duration(mult)
}

// retention is how long an idle key is remembered before its failures decay to
// zero — the maximum lockout, so a key is never dropped while still locked.
func (t *Throttle) retention() time.Duration {
	return t.baseLockout * maxBackoffMultiplier
}

// isExpired reports whether st is past its lockout and has sat idle beyond the
// retention window, making it safe to reset or evict.
func (t *Throttle) isExpired(st *attemptState, now time.Time) bool {
	return now.After(st.lockedUntil) && now.Sub(st.lastSeen) > t.retention()
}

// sweepLocked drops expired entries once the map grows past the soft cap. The
// caller must hold t.mu.
func (t *Throttle) sweepLocked(now time.Time) {
	if len(t.states) <= throttleSoftCap {
		return
	}

	for k, st := range t.states {
		if t.isExpired(st, now) {
			delete(t.states, k)
		}
	}
}

// WriteThrottled sends a 429 with a Retry-After header derived from the
// remaining lockout.
func WriteThrottled(w xhttp.JSONResponseWriter, retryAfter time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds(retryAfter)))
	w.WriteErrorMsg(http.StatusTooManyRequests, "too many attempts, please retry later")
}

// retryAfterSeconds rounds a lockout up to whole seconds, never below 1.
func retryAfterSeconds(d time.Duration) int {
	secs := int((d + time.Second - 1) / time.Second)
	if secs < 1 {
		return 1
	}

	return secs
}

// ClientIP resolves the client address for rate-limit keying. It uses the
// connection's RemoteAddr by default; only when trustProxy is set (the operator
// runs a reverse proxy that populates it) does it honor the leftmost
// X-Forwarded-For hop — trusting that header unconditionally would let a caller
// spoof its way around the per-IP limit.
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if first := strings.TrimSpace(strings.Split(xff, ",")[0]); first != "" {
				return first
			}
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
