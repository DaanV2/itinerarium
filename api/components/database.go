package components

import (
	"fmt"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
)

// SetupDatabase opens the configured database backend and applies migrations.
// The returned *persistence.Database participates in the shutdown lifecycle.
func SetupDatabase(cfg *ServerConfig) (*persistence.Database, error) {
	opts, err := databaseOptions(cfg.Database)
	if err != nil {
		return nil, err
	}

	db, err := persistence.New(opts...)
	if err != nil {
		return nil, err
	}

	if err := db.Migrate(); err != nil {
		return nil, err
	}

	return db, nil
}

// databaseOptions translates the resolved DatabaseConfig into persistence
// options, validating the backend and its required settings.
func databaseOptions(cfg DatabaseConfig) ([]persistence.Option, error) {
	dbType := persistence.DBType(cfg.Type)
	if !dbType.Valid() {
		return nil, fmt.Errorf("unsupported database type %q (want sqlite, memory, postgres, or mysql)", cfg.Type)
	}

	opts := []persistence.Option{
		persistence.WithType(dbType),
		persistence.WithMaxIdleConns(cfg.MaxIdleConns),
		persistence.WithMaxOpenConns(cfg.MaxOpenConns),
		persistence.WithConnMaxLifetime(cfg.ConnMaxLifetime),
	}

	switch dbType {
	case persistence.SQLite:
		// DSN wins over the file path when explicitly set.
		dsn := cfg.DSN
		if dsn == "" {
			dsn = cfg.Path
		}

		opts = append(opts, persistence.WithDSN(dsn))
	case persistence.PostgreSQL, persistence.MySQL:
		if cfg.DSN == "" {
			return nil, fmt.Errorf("database type %q requires --database-dsn (a connection string)", cfg.Type)
		}

		opts = append(opts, persistence.WithDSN(cfg.DSN))
	case persistence.InMemory:
		// No DSN needed; the backend is ephemeral.
	}

	return opts, nil
}
