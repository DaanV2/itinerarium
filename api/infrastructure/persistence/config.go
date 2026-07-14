package persistence

import (
	"fmt"
	"time"

	"github.com/DaanV2/itinerarium/api/infrastructure/config"
)

// DatabaseConfigSet groups the database backend flags. The set is declared
// here, next to the code that consumes it; commands opt in with AddToSet.
var (
	DatabaseConfigSet = config.New("database").WithValidate(validateDatabaseFlags)

	TypeFlag = DatabaseConfigSet.String("database.type", SQLite.String(),
		"database backend: sqlite, memory, postgres, or mysql")
	DSNFlag = DatabaseConfigSet.String("database.dsn", "",
		"database connection string (postgres/mysql); overrides database.path for sqlite")
	PathFlag = DatabaseConfigSet.String("database.path", "data/itinerarium.db",
		"path to the SQLite database file (sqlite backend)")
	MaxIdleConnsFlag = DatabaseConfigSet.Int("database.max-idle-conns", 2,
		"maximum number of idle connections in the pool")
	MaxOpenConnsFlag = DatabaseConfigSet.Int("database.max-open-conns", 0,
		"maximum number of open connections (0 = unlimited)")
	ConnMaxLifetimeFlag = DatabaseConfigSet.Duration("database.conn-max-lifetime", time.Hour,
		"maximum amount of time a connection may be reused")
)

func validateDatabaseFlags(c *config.Config) error {
	if t := c.GetString("database.type"); !DBType(t).Valid() {
		return fmt.Errorf("unsupported database type %q (want sqlite, memory, postgres, or mysql)", t)
	}

	return nil
}

// DatabaseConfig is a resolved snapshot of the database settings.
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

// GetConfig reads the database flags (flags → env → YAML → defaults) into a
// DatabaseConfig.
func GetConfig() DatabaseConfig {
	return DatabaseConfig{
		Type:            TypeFlag.Value(),
		DSN:             DSNFlag.Value(),
		Path:            PathFlag.Value(),
		MaxIdleConns:    MaxIdleConnsFlag.Value(),
		MaxOpenConns:    MaxOpenConnsFlag.Value(),
		ConnMaxLifetime: ConnMaxLifetimeFlag.Value(),
	}
}

// GetOptions translates the resolved database flags into options for [New].
func GetOptions() ([]Option, error) {
	return GetConfig().Options()
}

// Options translates the snapshot into options for [New], validating the
// backend and its required settings.
func (cfg DatabaseConfig) Options() ([]Option, error) {
	dbType := DBType(cfg.Type)
	if !dbType.Valid() {
		return nil, fmt.Errorf("unsupported database type %q (want sqlite, memory, postgres, or mysql)", cfg.Type)
	}

	opts := []Option{
		WithType(dbType),
		WithMaxIdleConns(cfg.MaxIdleConns),
		WithMaxOpenConns(cfg.MaxOpenConns),
		WithConnMaxLifetime(cfg.ConnMaxLifetime),
	}

	switch dbType {
	case SQLite:
		// DSN wins over the file path when explicitly set.
		dsn := cfg.DSN
		if dsn == "" {
			dsn = cfg.Path
		}

		opts = append(opts, WithDSN(dsn))
	case PostgreSQL, MySQL:
		if cfg.DSN == "" {
			return nil, fmt.Errorf("database type %q requires --database.dsn (a connection string)", cfg.Type)
		}

		opts = append(opts, WithDSN(cfg.DSN))
	case InMemory:
		// No DSN needed; the backend is ephemeral.
	}

	return opts, nil
}
