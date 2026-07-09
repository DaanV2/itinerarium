package components

import (
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
)

// SetupAuthentication loads (or generates on first run) the RS512 signing key
// pair and builds the token service that issues and revokes JWTs. The
// revocation store is a repository from NewRepositories.
func SetupAuthentication(
	cfg *ServerConfig,
	revocation authentication.RevocationStore,
) (*authentication.TokenService, error) {
	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(cfg.KeysPath))
	if err != nil {
		return nil, err
	}

	tokens := authentication.NewTokenService(
		keys,
		revocation,
		authentication.WithTTL(cfg.TokenTTL),
	)

	return tokens, nil
}
