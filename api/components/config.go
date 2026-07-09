package components

import (
	"fmt"
	"net"
	"time"

	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/config"
)

// Address is the host and port the API server listens on. A bare port (empty
// Host) listens on every interface.
type Address struct {
	Host string
	Port string
}

// ParseAddress splits a "host:port" string into an Address. ":8080" yields an
// empty host.
func ParseAddress(addr string) (Address, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return Address{}, fmt.Errorf("parsing address %q: %w", addr, err)
	}

	return Address{Host: host, Port: port}, nil
}

// Listen renders the address as the "host:port" string http.Server.Addr wants.
func (a Address) Listen() string {
	return net.JoinHostPort(a.Host, a.Port)
}

// ServerConfig is the resolved "server" configuration the components need. It
// centralizes the flag/env/YAML keys and their defaults in one place so every
// builder and command reads them the same way.
type ServerConfig struct {
	Address      Address
	DatabasePath string
	KeysPath     string
	TokenTTL     time.Duration
	CatalogPath  string
}

// LoadServerConfig reads the "server" config context (flags → env → YAML →
// defaults) into a ServerConfig.
func LoadServerConfig() (*ServerConfig, error) {
	cfg := config.GetContext("server")

	address, err := ParseAddress(cfg.String("address", ":8080"))
	if err != nil {
		return nil, err
	}

	return &ServerConfig{
		Address:      address,
		DatabasePath: cfg.String("database-path", "data/itinerarium.db"),
		KeysPath:     cfg.String("keys-path", "data/keys"),
		TokenTTL:     cfg.Duration("token-ttl", authentication.DefaultTokenTTL),
		CatalogPath:  cfg.String("catalog-path", ""),
	}, nil
}
