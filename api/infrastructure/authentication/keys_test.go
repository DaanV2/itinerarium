package authentication_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
)

func TestNewKeyStore_GeneratesOnFirstStart(t *testing.T) {
	dir := t.TempDir()

	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(dir))
	if err != nil {
		t.Fatalf("NewKeyStore: %v", err)
	}

	if keys.PrivateKey() == nil || keys.PublicKey() == nil {
		t.Fatal("expected a generated key pair")
	}

	for _, name := range []string{"private.pem", "public.pem"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s to be written: %v", name, err)
		}
	}
}

func TestNewKeyStore_LoadsExistingKeys(t *testing.T) {
	dir := t.TempDir()

	first, err := authentication.NewKeyStore(authentication.WithKeysDir(dir))
	if err != nil {
		t.Fatalf("NewKeyStore (generate): %v", err)
	}

	second, err := authentication.NewKeyStore(authentication.WithKeysDir(dir))
	if err != nil {
		t.Fatalf("NewKeyStore (load): %v", err)
	}

	if !first.PrivateKey().Equal(second.PrivateKey()) {
		t.Fatal("expected the second start to load the same key pair, got a different one")
	}
}
