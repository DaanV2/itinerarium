// Package persistence owns the database connection and migrations. All query
// code lives in the repositories subpackage — services never touch *gorm.DB
// directly.
package persistence

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite" // pure-Go SQLite driver (no cgo)
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DBType selects the database backend. SQLite is the default, self-hosted
// friendly choice; postgres and mysql exist for larger deployments, and memory
// is a throwaway SQLite for tests.
type DBType string

const (
	// SQLite is a pure-Go, file-backed database (the default).
	SQLite DBType = "sqlite"
	// InMemory is an ephemeral SQLite database — data is lost on shutdown.
	InMemory DBType = "memory"
	// PostgreSQL connects to a PostgreSQL server via a DSN.
	PostgreSQL DBType = "postgres"
	// MySQL connects to a MySQL/MariaDB server via a DSN.
	MySQL DBType = "mysql"
)

func (t DBType) String() string { return string(t) }

// Valid reports whether t names a supported backend.
func (t DBType) Valid() bool {
	switch t {
	case SQLite, InMemory, PostgreSQL, MySQL:
		return true
	default:
		return false
	}
}

// Database wraps the GORM connection and participates in the shutdown
// lifecycle.
type Database struct {
	db *gorm.DB
}

type settings struct {
	dbType          DBType
	dsn             string
	maxIdleConns    int
	maxOpenConns    int
	connMaxLifetime time.Duration
	config          *gorm.Config
}

// Option configures New via the functional-options pattern.
type Option func(*settings)

// WithType selects the database backend. Pair it with WithDSN for
// postgres/mysql, or WithPath for sqlite.
func WithType(t DBType) Option {
	return func(s *settings) { s.dbType = t }
}

// WithDSN sets the data source name: a connection string for postgres/mysql,
// or a file path for sqlite.
func WithDSN(dsn string) Option {
	return func(s *settings) { s.dsn = dsn }
}

// WithPath points a SQLite database at a file; parent directories are created
// as needed. Shorthand for WithType(SQLite) + WithDSN(path).
func WithPath(path string) Option {
	return func(s *settings) {
		s.dbType = SQLite
		s.dsn = path
	}
}

// WithInMemory backs the database with memory only. Use this in tests.
func WithInMemory() Option {
	return func(s *settings) { s.dbType = InMemory }
}

// WithMaxIdleConns sets the maximum number of idle connections in the pool.
func WithMaxIdleConns(n int) Option {
	return func(s *settings) { s.maxIdleConns = n }
}

// WithMaxOpenConns sets the maximum number of open connections (0 = unlimited).
func WithMaxOpenConns(n int) Option {
	return func(s *settings) { s.maxOpenConns = n }
}

// WithConnMaxLifetime sets the maximum time a connection may be reused.
func WithConnMaxLifetime(d time.Duration) Option {
	return func(s *settings) { s.connMaxLifetime = d }
}

// New opens the database. Defaults to a SQLite file at data/itinerarium.db
// when no option overrides the type or path.
func New(opts ...Option) (*Database, error) {
	s := &settings{
		dbType:          SQLite,
		dsn:             filepath.Join("data", "itinerarium.db"),
		maxIdleConns:    2,
		maxOpenConns:    0,
		connMaxLifetime: time.Hour,
		config: &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		},
	}
	for _, opt := range opts {
		opt(s)
	}

	dialector, err := s.dialector()
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(dialector, s.config)
	if err != nil {
		return nil, fmt.Errorf("opening %s database: %w", s.dbType, err)
	}

	if err := applyPool(db, s); err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

// dialector picks the GORM driver for the configured backend, preparing the
// filesystem for file-backed SQLite along the way.
func (s *settings) dialector() (gorm.Dialector, error) {
	switch s.dbType {
	case SQLite:
		if err := ensureParentDir(s.dsn); err != nil {
			return nil, err
		}

		return sqlite.Open(s.dsn), nil
	case InMemory:
		return sqlite.Open(":memory:"), nil
	case PostgreSQL:
		return postgres.Open(s.dsn), nil
	case MySQL:
		return mysql.Open(s.dsn), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %q", s.dbType)
	}
}

// applyPool configures the underlying sql.DB connection pool.
func applyPool(db *gorm.DB, s *settings) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("accessing sql.DB: %w", err)
	}

	sqlDB.SetMaxIdleConns(s.maxIdleConns)
	sqlDB.SetMaxOpenConns(s.maxOpenConns)
	sqlDB.SetConnMaxLifetime(s.connMaxLifetime)

	return nil
}

// ensureParentDir creates the directory holding a SQLite database file.
func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}

	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating database directory: %w", err)
	}

	return nil
}

// DB exposes the underlying GORM handle for repositories.
func (d *Database) DB() *gorm.DB { return d.db }

// Shutdown closes the underlying connection (lifecycle.Shutdown phase).
func (d *Database) Shutdown(_ context.Context) error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}
