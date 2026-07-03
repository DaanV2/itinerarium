package authentication_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
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
	if err != nil {
		t.Fatalf("NewKeyStore: %v", err)
	}

	return authentication.NewTokenService(keys, newFakeRevocationStore(), opts...)
}

func TestTokenService_IssueAndParse(t *testing.T) {
	svc := newTestTokenService(t)

	token, err := svc.Issue("user-1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	claims, err := svc.Parse(t.Context(), token)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if claims.Subject != "user-1" {
		t.Fatalf("subject = %q, want user-1", claims.Subject)
	}
	if claims.ID == "" {
		t.Fatal("expected a non-empty JTI")
	}
}

func TestTokenService_Parse_RejectsExpired(t *testing.T) {
	svc := newTestTokenService(t, authentication.WithTTL(-time.Minute))

	token, err := svc.Issue("user-1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if _, err := svc.Parse(t.Context(), token); err == nil {
		t.Fatal("expected an expired token to be rejected")
	}
}

func TestTokenService_Parse_RejectsRevoked(t *testing.T) {
	svc := newTestTokenService(t)

	token, err := svc.Issue("user-1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if err := svc.Revoke(t.Context(), token); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	_, err = svc.Parse(t.Context(), token)
	if !errors.Is(err, authentication.ErrRevoked) {
		t.Fatalf("Parse after revoke: got %v, want ErrRevoked", err)
	}
}

func TestTokenService_Parse_RejectsForgedSignature(t *testing.T) {
	svc := newTestTokenService(t)
	other := newTestTokenService(t)

	token, err := other.Issue("user-1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if _, err := svc.Parse(t.Context(), token); err == nil {
		t.Fatal("expected a token signed by a different key pair to be rejected")
	}
}
