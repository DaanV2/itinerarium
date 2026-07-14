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

// addDatabaseFlags registers the database backend flags on a command. Shared
// by serve and init so headless deployments can point either at
// postgres/mysql. It does not bind the flags — see [bindDatabaseFlags].
func addDatabaseFlags(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.String("database-type", persistence.SQLite.String(), "database backend: sqlite, memory, postgres, or mysql")
	fs.String("database-dsn", "", "database connection string (postgres/mysql); overrides --database-path for sqlite")
	fs.String("database-path", "data/itinerarium.db", "path to the SQLite database file (sqlite backend)")
	fs.Int("database-max-idle-conns", 2, "maximum number of idle connections in the pool")
	fs.Int("database-max-open-conns", 0, "maximum number of open connections (0 = unlimited)")
	fs.Duration("database-conn-max-lifetime", time.Hour, "maximum amount of time a connection may be reused")
}

// bindDatabaseFlags binds cmd's own database flags under the "server" config
// context, for whichever of them cmd actually declares (see
// [addDatabaseFlags]). Called from rootCmd's PersistentPreRunE, once the
// invoked leaf command is known — not from package init() time, since
// init() runs for every command file regardless of which subcommand is
// invoked, and multiple commands registering their own flag objects under
// the same "server.database-*" keys would let whichever command's flags
// bind last silently win the config lookup for every other command.
func bindDatabaseFlags(cmd *cobra.Command) {
	fs := cmd.Flags()
	for _, name := range databaseFlags {
		if f := fs.Lookup(name); f != nil {
			config.MustBindFlag("server", f)
		}
	}
}
