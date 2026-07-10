package components

import (
	"fmt"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
)

// SetupDatabase opens the configured database backend and applies migrations.
// The returned *persistence.Database participates in the shutdown lifecycle.
func SetupDatabase(cfg *ServerConfig) (*persistence.Database, error) {
	opts, err := databaseOptions(cfg)
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

// databaseOptions translates the resolved ServerConfig into persistence
// options, validating the backend and its required settings.
func databaseOptions(cfg *ServerConfig) ([]persistence.Option, error) {
	dbType := persistence.DBType(cfg.DatabaseType)
	if !dbType.Valid() {
		return nil, fmt.Errorf("unsupported database type %q (want sqlite, memory, postgres, or mysql)", cfg.DatabaseType)
	}

	opts := []persistence.Option{
		persistence.WithType(dbType),
		persistence.WithMaxIdleConns(cfg.DatabaseMaxIdleConns),
		persistence.WithMaxOpenConns(cfg.DatabaseMaxOpenConns),
		persistence.WithConnMaxLifetime(cfg.DatabaseConnMaxLifetime),
	}

	switch dbType {
	case persistence.SQLite:
		// DSN wins over the file path when explicitly set.
		dsn := cfg.DatabaseDSN
		if dsn == "" {
			dsn = cfg.DatabasePath
		}

		opts = append(opts, persistence.WithDSN(dsn))
	case persistence.PostgreSQL, persistence.MySQL:
		if cfg.DatabaseDSN == "" {
			return nil, fmt.Errorf("database type %q requires --database-dsn (a connection string)", cfg.DatabaseType)
		}

		opts = append(opts, persistence.WithDSN(cfg.DatabaseDSN))
	case persistence.InMemory:
		// No DSN needed; the backend is ephemeral.
	}

	return opts, nil
}
