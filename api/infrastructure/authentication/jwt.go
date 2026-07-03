package authentication

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// DefaultTokenTTL is how long an issued access token remains valid absent an
// explicit override.
const DefaultTokenTTL = 15 * time.Minute

// ErrRevoked is returned by Parse when the token's JTI has been revoked.
var ErrRevoked = errors.New("token revoked")

// Claims is the RS512 JWT payload. Subject carries the user ID, ID carries
// the JTI used for revocation lookups.
type Claims struct {
	jwt.RegisteredClaims
}

// RevocationStore records and checks revoked JTIs. Implemented by
// infrastructure/persistence/repositories.RevokedTokens.
type RevocationStore interface {
	Revoke(ctx context.Context, jti string, expiresAt time.Time) error
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

// TokenService issues and validates RS512 JWTs and enforces JTI revocation.
type TokenService struct {
	keys       *KeyStore
	revocation RevocationStore
	ttl        time.Duration
	issuer     string
}

// TokenOption configures NewTokenService via the functional-options pattern.
type TokenOption func(*TokenService)

// WithTTL overrides the access token lifetime (default DefaultTokenTTL).
func WithTTL(ttl time.Duration) TokenOption {
	return func(s *TokenService) { s.ttl = ttl }
}

// WithIssuer sets the "iss" claim on issued tokens (default "itinerarium").
func WithIssuer(issuer string) TokenOption {
	return func(s *TokenService) { s.issuer = issuer }
}

// NewTokenService builds a token service around a key pair and a revocation
// store.
func NewTokenService(keys *KeyStore, revocation RevocationStore, opts ...TokenOption) *TokenService {
	s := &TokenService{
		keys:       keys,
		revocation: revocation,
		ttl:        DefaultTokenTTL,
		issuer:     "itinerarium",
	}
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Issue signs a new RS512 JWT for the given subject (typically a user ID)
// with a fresh JTI.
func (s *TokenService) Issue(subject string) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ID:        uuid.NewString(),
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodRS512, claims).SignedString(s.keys.PrivateKey())
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}

	return token, nil
}

// Parse validates signature and expiry, then rejects the token if its JTI has
// been revoked.
func (s *TokenService) Parse(ctx context.Context, tokenString string) (*Claims, error) {
	claims, err := s.verify(tokenString)
	if err != nil {
		return nil, err
	}

	revoked, err := s.revocation.IsRevoked(ctx, claims.ID)
	if err != nil {
		return nil, fmt.Errorf("checking revocation: %w", err)
	}
	if revoked {
		return nil, ErrRevoked
	}

	return claims, nil
}

// Revoke validates the token's signature, then records its JTI as revoked
// until the token's original expiry (logout, credential reset).
func (s *TokenService) Revoke(ctx context.Context, tokenString string) error {
	claims, err := s.verify(tokenString)
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(s.ttl)
	if claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Time
	}

	if err := s.revocation.Revoke(ctx, claims.ID, expiresAt); err != nil {
		return fmt.Errorf("revoking token: %w", err)
	}

	return nil
}

func (s *TokenService) verify(tokenString string) (*Claims, error) {
	claims := &Claims{}

	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodRS512 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}

		return s.keys.PublicKey(), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}

	return claims, nil
}
