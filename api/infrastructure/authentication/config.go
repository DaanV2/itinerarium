package authentication

import (
	"fmt"

	"github.com/DaanV2/itinerarium/api/infrastructure/config"
)

// AuthConfigSet groups the authentication flags. The set is declared here,
// next to the code that consumes it; commands opt in with AddToSet.
var (
	AuthConfigSet = config.New("auth").WithValidate(validateAuthFlags)

	KeysPathFlag = AuthConfigSet.String("auth.keys-path", "data/keys",
		"directory holding the RS512 JWT signing key pair")
	TokenTTLFlag = AuthConfigSet.Duration("auth.token-ttl", DefaultTokenTTL,
		"access token lifetime")
)

func validateAuthFlags(c *config.Config) error {
	if ttl := c.GetDuration("auth.token-ttl"); ttl <= 0 {
		return fmt.Errorf("auth.token-ttl must be positive, got %s", ttl)
	}

	return nil
}
