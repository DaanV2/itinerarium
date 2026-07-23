package components

import (
	"time"

	"github.com/DaanV2/itinerarium/api/infrastructure/config"
)

// CatalogConfigSet groups the catalog seeding flags, consumed by seedCatalog
// in BuildServer. Commands opt in with AddToSet.
var (
	CatalogConfigSet = config.New("catalog")

	CatalogPathFlag = CatalogConfigSet.String("catalog.path", "",
		"optional JSON/YAML file seeding the currency and item catalog on startup")
)

// SecurityConfigSet groups the HTTP-hardening flags (M10): request body-size
// limit, CSP, and HSTS. Consumed by CreateRouter. Commands opt in with AddToSet.
var (
	SecurityConfigSet = config.New("security")

	BodyLimitFlag = SecurityConfigSet.Int("security.body-limit", 10<<20,
		"maximum accepted request body size in bytes (0 disables the limit)")

	CSPFlag = SecurityConfigSet.String("security.csp", "",
		"Content-Security-Policy header value (empty uses a built-in policy for the embedded SPA)")
	HSTSFlag = SecurityConfigSet.Bool("security.hsts", false,
		"send Strict-Transport-Security on every response (enable when served behind TLS)")

	// Login/reset rate limiting (M10). LoginMaxFailures <= 0 disables the
	// in-process limiter entirely (e.g. when a reverse proxy handles throttling).
	LoginMaxFailuresFlag = SecurityConfigSet.Int("security.login-max-failures", 5,
		"failed login attempts (per IP and per account) before a lockout; 0 disables in-process rate limiting")
	LoginLockoutFlag = SecurityConfigSet.Duration("security.login-lockout", time.Minute,
		"base lockout after too many failed logins; doubles per further failure up to 32x")
	TrustProxyHeadersFlag = SecurityConfigSet.Bool("security.trust-proxy-headers", false,
		"trust X-Forwarded-For for the client IP in rate limiting (enable only behind a trusted reverse proxy)")
)
