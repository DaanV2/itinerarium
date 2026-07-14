package servers

import (
	"fmt"
	"net"

	"github.com/DaanV2/itinerarium/api/infrastructure/config"
)

// ServerConfigSet groups the HTTP server flags. The set is declared here,
// next to the code that consumes it; commands opt in with AddToSet.
var (
	ServerConfigSet = config.New("server").WithValidate(validateServerFlags)

	AddressFlag = ServerConfigSet.String("server.address", ":8080",
		"address the API server listens on")
)

func validateServerFlags(c *config.Config) error {
	addr := c.GetString("server.address")
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return fmt.Errorf("parsing server.address %q: %w", addr, err)
	}

	return nil
}
