package transport_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DaanV2/itinerarium/api/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLoginThrottle_DisabledWhenNonPositive(t *testing.T) {
	assert.Nil(t, transport.NewLoginThrottle(0, time.Minute), "0 max failures disables the limiter")
	assert.Nil(t, transport.NewLoginThrottle(-1, time.Minute), "a negative threshold disables the limiter")
}

func TestThrottle_AllowsUntilThreshold(t *testing.T) {
	now := time.Now()
	throttle := transport.NewTestThrottle(3, time.Minute, func() time.Time { return now })

	for i := range 2 {
		throttle.Penalize("k")
		allowed, _ := throttle.Allowed("k")
		require.Truef(t, allowed, "still allowed after %d failures (below threshold)", i+1)
	}

	throttle.Penalize("k") // third failure reaches the threshold
	allowed, retry := throttle.Allowed("k")
	assert.False(t, allowed, "the threshold failure locks the key")
	assert.Positive(t, retry, "a locked key reports a remaining lockout")
}

func TestThrottle_ResetClearsLock(t *testing.T) {
	now := time.Now()
	throttle := transport.NewTestThrottle(2, time.Minute, func() time.Time { return now })

	throttle.Penalize("k")
	throttle.Penalize("k")
	allowed, _ := throttle.Allowed("k")
	require.False(t, allowed, "precondition: key is locked")

	throttle.Reset("k")
	allowed, _ = throttle.Allowed("k")
	assert.True(t, allowed, "Reset clears the lockout")
}

func TestThrottle_RecoversAfterClockAdvance(t *testing.T) {
	now := time.Now()
	throttle := transport.NewTestThrottle(1, time.Minute, func() time.Time { return now })

	throttle.Penalize("k")
	allowed, _ := throttle.Allowed("k")
	require.False(t, allowed, "precondition: locked after the base lockout is set")

	now = now.Add(90 * time.Second) // past the 1-minute base lockout
	allowed, _ = throttle.Allowed("k")
	assert.True(t, allowed, "the lockout expires once the clock passes it")
}

func TestThrottle_BackoffGrowsAndCaps(t *testing.T) {
	now := time.Now()
	throttle := transport.NewTestThrottle(1, time.Second, func() time.Time { return now })

	throttle.Penalize("k")
	_, first := throttle.Allowed("k")
	assert.Equal(t, time.Second, first, "the first lockout is the base duration")

	var last time.Duration
	for range 10 {
		throttle.Penalize("k")
		_, last = throttle.Allowed("k")
	}

	assert.Equal(t, 32*time.Second, last, "a sustained attack should grow to — and cap at — 32x the base")
}

func TestThrottle_NilIsAlwaysAllowed(t *testing.T) {
	var throttle *transport.Throttle

	allowed, retry := throttle.Allowed("k")
	assert.True(t, allowed)
	assert.Zero(t, retry)

	// Penalize/Reset on a nil throttle must be safe no-ops.
	throttle.Penalize("k")
	throttle.Reset("k")

	allowed, _ = throttle.Allowed("k")
	assert.True(t, allowed, "a nil throttle never locks")
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "203.0.113.9:5555"
	req.Header.Set("X-Forwarded-For", "198.51.100.7, 10.0.0.1")

	assert.Equal(t, "203.0.113.9", transport.ClientIP(req, false), "must ignore X-Forwarded-For when the proxy isn't trusted")
	assert.Equal(t, "198.51.100.7", transport.ClientIP(req, true), "must use the leftmost X-Forwarded-For hop when trusted")
}
