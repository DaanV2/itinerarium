// Package persistence owns the database connection and migrations. All query
// code lives in the repositories subpackage — services never touch *gorm.DB
// directly.
package persistence

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite" // pure-Go SQLite driver (no cgo), FTS5-capable
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Database wraps the GORM connection and participates in the shutdown
// lifecycle.
type Database struct {
	db *gorm.DB
}

type settings struct {
	dsn    string
	config *gorm.Config
}

// Option configures New via the functional-options pattern.
type Option func(*settings)

// WithPath points the database at a SQLite file; parent directories are
// created as needed.
func WithPath(path string) Option {
	return func(s *settings) { s.dsn = path }
}

// WithInMemory backs the database with memory only. Use this in tests.
func WithInMemory() Option {
	return func(s *settings) { s.dsn = ":memory:" }
}

// New opens the database. Defaults to data/itinerarium.db when no option
// overrides the path.
func New(opts ...Option) (*Database, error) {
	s := &settings{
		dsn: filepath.Join("data", "itinerarium.db"),
		config: &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		},
	}
	for _, opt := range opts {
		opt(s)
	}

	if s.dsn != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(s.dsn), 0o750); err != nil {
			return nil, fmt.Errorf("creating database directory: %w", err)
		}
	}

	db, err := gorm.Open(sqlite.Open(s.dsn), s.config)
	if err != nil {
		return nil, fmt.Errorf("opening database %q: %w", s.dsn, err)
	}

	return &Database{db: db}, nil
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
