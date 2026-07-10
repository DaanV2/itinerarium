package components

import (
	"fmt"
	"net"
	"time"

	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/config"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
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

// DatabaseConfig is the resolved database configuration for a backend.
type DatabaseConfig struct {
	// Type selects the backend: sqlite (default), memory, postgres, mysql.
	Type string
	// DSN is the connection string for postgres/mysql. For sqlite it overrides
	// Path when set.
	DSN string
	// Path is the sqlite file location (used when DSN is empty).
	Path            string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
}

// ServerConfig is the resolved "server" configuration the components need. It
// centralizes the flag/env/YAML keys and their defaults in one place so every
// builder and command reads them the same way.
type ServerConfig struct {
	Address     Address
	Database    DatabaseConfig
	KeysPath    string
	TokenTTL    time.Duration
	CatalogPath string
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
		Address: address,
		Database: DatabaseConfig{
			Type:            cfg.String("database-type", persistence.SQLite.String()),
			DSN:             cfg.String("database-dsn", ""),
			Path:            cfg.String("database-path", "data/itinerarium.db"),
			MaxIdleConns:    cfg.Int("database-max-idle-conns", 2),
			MaxOpenConns:    cfg.Int("database-max-open-conns", 0),
			ConnMaxLifetime: cfg.Duration("database-conn-max-lifetime", time.Hour),
		},
		KeysPath:    cfg.String("keys-path", "data/keys"),
		TokenTTL:    cfg.Duration("token-ttl", authentication.DefaultTokenTTL),
		CatalogPath: cfg.String("catalog-path", ""),
	}, nil
}
