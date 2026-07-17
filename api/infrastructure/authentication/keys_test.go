package authentication_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/stretchr/testify/require"
)

func TestNewKeyStore_GeneratesOnFirstStart(t *testing.T) {
	dir := t.TempDir()

	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(dir))
	require.NoError(t, err, "NewKeyStore")

	require.NotNil(t, keys.PrivateKey(), "expected a generated key pair")
	require.NotNil(t, keys.PublicKey(), "expected a generated key pair")

	for _, name := range []string{"private.pem", "public.pem"} {
		_, err := os.Stat(filepath.Join(dir, name))
		require.NoError(t, err, "expected %s to be written", name)
	}
}

func TestNewKeyStore_LoadsExistingKeys(t *testing.T) {
	dir := t.TempDir()

	first, err := authentication.NewKeyStore(authentication.WithKeysDir(dir))
	require.NoError(t, err, "NewKeyStore (generate)")

	second, err := authentication.NewKeyStore(authentication.WithKeysDir(dir))
	require.NoError(t, err, "NewKeyStore (load)")

	require.True(t, first.PrivateKey().Equal(second.PrivateKey()),
		"expected the second start to load the same key pair, got a different one")
}
