package cmd

import (
	"time"

	"github.com/DaanV2/itinerarium/api/infrastructure/config"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/spf13/cobra"
)

// databaseFlags names the flags addDatabaseFlags registers, so they can be
// bound individually under the "server" config namespace without pulling the
// command's other flags along.
var databaseFlags = []string{
	"database-type",
	"database-dsn",
	"database-path",
	"database-max-idle-conns",
	"database-max-open-conns",
	"database-conn-max-lifetime",
}

// addDatabaseFlags registers the database backend flags on a command and binds
// them under the "server" config context. Shared by serve and init so headless
// deployments can point either at postgres/mysql.
func addDatabaseFlags(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.String("database-type", persistence.SQLite.String(), "database backend: sqlite, memory, postgres, or mysql")
	fs.String("database-dsn", "", "database connection string (postgres/mysql); overrides --database-path for sqlite")
	fs.String("database-path", "data/itinerarium.db", "path to the SQLite database file (sqlite backend)")
	fs.Int("database-max-idle-conns", 2, "maximum number of idle connections in the pool")
	fs.Int("database-max-open-conns", 0, "maximum number of open connections (0 = unlimited)")
	fs.Duration("database-conn-max-lifetime", time.Hour, "maximum amount of time a connection may be reused")

	for _, name := range databaseFlags {
		config.MustBindFlag("server", fs.Lookup(name))
	}
}
