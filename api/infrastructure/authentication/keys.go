// Package authentication issues and validates RS512 JWTs, manages the RSA
// signing key pair (auto-generated on first start), hashes passwords, and
// defines the JTI revocation contract implemented by the persistence layer.
package authentication

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

const (
	keyBits         = 2048
	privateKeyFile  = "private.pem"
	publicKeyFile   = "public.pem"
	privateFileMode = 0o600
	publicFileMode  = 0o644
	keysDirMode     = 0o750
)

// KeyStore holds the RSA key pair used to sign and verify JWTs. Keys are
// generated on first use and persisted to disk; subsequent starts load the
// existing pair.
type KeyStore struct {
	private *rsa.PrivateKey
	public  *rsa.PublicKey
}

type keySettings struct {
	dir string
}

// KeyOption configures NewKeyStore via the functional-options pattern.
type KeyOption func(*keySettings)

// WithKeysDir points the key store at a directory; it is created as needed.
func WithKeysDir(dir string) KeyOption {
	return func(s *keySettings) { s.dir = dir }
}

// NewKeyStore loads the RSA key pair from disk, generating and persisting a
// new one on first start.
func NewKeyStore(opts ...KeyOption) (*KeyStore, error) {
	s := &keySettings{dir: filepath.Join("data", "keys")}
	for _, opt := range opts {
		opt(s)
	}

	privatePath := filepath.Join(s.dir, privateKeyFile)

	_, err := os.Stat(privatePath)
	switch {
	case err == nil:
		return loadKeyStore(privatePath)
	case os.IsNotExist(err):
		return generateKeyStore(s.dir, privatePath, filepath.Join(s.dir, publicKeyFile))
	default:
		return nil, fmt.Errorf("checking for existing key pair: %w", err)
	}
}

func generateKeyStore(dir, privatePath, publicPath string) (*KeyStore, error) {
	if err := os.MkdirAll(dir, keysDirMode); err != nil {
		return nil, fmt.Errorf("creating keys directory: %w", err)
	}

	key, err := rsa.GenerateKey(rand.Reader, keyBits)
	if err != nil {
		return nil, fmt.Errorf("generating RSA key pair: %w", err)
	}

	privateBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	if err := os.WriteFile(privatePath, pem.EncodeToMemory(privateBlock), privateFileMode); err != nil {
		return nil, fmt.Errorf("writing private key: %w", err)
	}

	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshalling public key: %w", err)
	}

	publicBlock := &pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}
	if err := os.WriteFile(publicPath, pem.EncodeToMemory(publicBlock), publicFileMode); err != nil {
		return nil, fmt.Errorf("writing public key: %w", err)
	}

	return &KeyStore{private: key, public: &key.PublicKey}, nil
}

func loadKeyStore(privatePath string) (*KeyStore, error) {
	privBytes, err := os.ReadFile(privatePath) //nolint:gosec // privatePath is built from operator-supplied config (flag/env/yaml), not user input
	if err != nil {
		return nil, fmt.Errorf("reading private key: %w", err)
	}

	block, _ := pem.Decode(privBytes)
	if block == nil {
		return nil, fmt.Errorf("decoding private key PEM from %s", privatePath)
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing private key: %w", err)
	}

	return &KeyStore{private: key, public: &key.PublicKey}, nil
}

// PrivateKey returns the RSA private key used to sign tokens.
func (k *KeyStore) PrivateKey() *rsa.PrivateKey { return k.private }

// PublicKey returns the RSA public key used to verify tokens.
func (k *KeyStore) PublicKey() *rsa.PublicKey { return k.public }
