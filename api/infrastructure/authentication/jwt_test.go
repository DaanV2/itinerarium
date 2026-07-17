package authentication_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/stretchr/testify/require"
)

// fakeRevocationStore is an in-memory authentication.RevocationStore for
// tests, standing in for repositories.RevokedTokens.
type fakeRevocationStore struct {
	mu      sync.Mutex
	revoked map[string]time.Time
}

func newFakeRevocationStore() *fakeRevocationStore {
	return &fakeRevocationStore{revoked: make(map[string]time.Time)}
}

func (f *fakeRevocationStore) Revoke(_ context.Context, jti string, expiresAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revoked[jti] = expiresAt

	return nil
}

func (f *fakeRevocationStore) IsRevoked(_ context.Context, jti string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.revoked[jti]

	return ok, nil
}

func newTestTokenService(t *testing.T, opts ...authentication.TokenOption) *authentication.TokenService {
	t.Helper()

	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(t.TempDir()))
	require.NoError(t, err, "NewKeyStore")

	return authentication.NewTokenService(keys, newFakeRevocationStore(), opts...)
}

func TestTokenService_IssueAndParse(t *testing.T) {
	svc := newTestTokenService(t)

	token, err := svc.Issue("user-1")
	require.NoError(t, err, "Issue")

	claims, err := svc.Parse(t.Context(), token)
	require.NoError(t, err, "Parse")

	require.Equal(t, "user-1", claims.Subject)
	require.NotEmpty(t, claims.ID, "expected a non-empty JTI")
}

func TestTokenService_Parse_RejectsExpired(t *testing.T) {
	svc := newTestTokenService(t, authentication.WithTTL(-time.Minute))

	token, err := svc.Issue("user-1")
	require.NoError(t, err, "Issue")

	_, err = svc.Parse(t.Context(), token)
	require.Error(t, err, "expected an expired token to be rejected")
}

func TestTokenService_Parse_RejectsRevoked(t *testing.T) {
	svc := newTestTokenService(t)

	token, err := svc.Issue("user-1")
	require.NoError(t, err, "Issue")

	require.NoError(t, svc.Revoke(t.Context(), token), "Revoke")

	_, err = svc.Parse(t.Context(), token)
	require.ErrorIs(t, err, authentication.ErrRevoked, "Parse after revoke")
}

func TestTokenService_Parse_RejectsForgedSignature(t *testing.T) {
	svc := newTestTokenService(t)
	other := newTestTokenService(t)

	token, err := other.Issue("user-1")
	require.NoError(t, err, "Issue")

	_, err = svc.Parse(t.Context(), token)
	require.Error(t, err, "expected a token signed by a different key pair to be rejected")
}
