// Package config wraps a Viper singleton. Resolution priority, highest first:
// command-line flags → environment variables → YAML config file → defaults.
//
// Every component reads its settings through a named [Context]:
//
//	cfg := config.GetContext("server")
//	addr := cfg.String("address", ":8080") // flag --address, env SERVER_ADDRESS, yaml server.address
//
// If no explicit file is passed to [Load], a config.yaml is searched for in
// the directories returned by [ConfigPaths].
package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	v        = newViper()
	contexts sync.Map // component name → *Context
)

func newViper() *viper.Viper {
	nv := viper.New()
	// "server.database-path" → env var SERVER_DATABASE_PATH
	nv.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	nv.AutomaticEnv()

	return nv
}

// Load reads a YAML config file into the singleton. With an explicit path,
// a missing or invalid file is an error. With an empty path, config.yaml is
// searched for across [ConfigPaths]; not finding one is not an error (flags,
// env vars, and defaults still apply).
func Load(file string) error {
	if file != "" {
		v.SetConfigFile(file)

		return v.ReadInConfig()
	}

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	for _, p := range ConfigPaths() {
		v.AddConfigPath(p)
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			return nil
		}

		return err
	}

	return nil
}

// ConfigPaths returns, in search order, the directories checked for a
// config.yaml when Load is called with an empty path.
func ConfigPaths() []string {
	paths := []string{".config"}

	if dir, err := os.UserConfigDir(); err == nil {
		paths = append(paths, filepath.Join(dir, "itinerarium"))
	}

	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".itinerarium"))
	}

	return paths
}

// Save writes the fully resolved configuration (defaults, env vars, and
// flags already applied) as YAML to the first entry in [ConfigPaths].
func Save() error {
	return SaveAs(filepath.Join(ConfigPaths()[0], "config.yaml"))
}

// SaveAs writes the fully resolved configuration as YAML to an explicit path,
// creating its parent directory if necessary.
func SaveAs(file string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		return err
	}

	return v.WriteConfigAs(file)
}

// BindFlags binds every flag in the set under the component's namespace, so
// flag --database-path on component "server" becomes key "server.database-path".
// Call it from the command's init() after defining the flags.
func BindFlags(component string, fs *pflag.FlagSet) error {
	var err error
	fs.VisitAll(func(f *pflag.Flag) {
		if e := v.BindPFlag(component+"."+f.Name, f); e != nil && err == nil {
			err = e
		}
	})

	return err
}

// MustBindFlags is BindFlags for init() paths where a bind error is a bug.
func MustBindFlags(component string, fs *pflag.FlagSet) {
	if err := BindFlags(component, fs); err != nil {
		panic(err)
	}
}

// Context is a component-scoped view on the config singleton.
type Context struct {
	component string
}

// GetContext returns the config context for a component, creating it on first
// use. Contexts are cached in a sync.Map and safe for concurrent use.
func GetContext(component string) *Context {
	c, _ := contexts.LoadOrStore(component, &Context{component: component})

	return c.(*Context)
}

func (c *Context) key(k string) string { return c.component + "." + k }

// String resolves a string setting, falling back to def.
func (c *Context) String(key, def string) string {
	v.SetDefault(c.key(key), def)

	return v.GetString(c.key(key))
}

// Int resolves an integer setting, falling back to def.
func (c *Context) Int(key string, def int) int {
	v.SetDefault(c.key(key), def)

	return v.GetInt(c.key(key))
}

// Bool resolves a boolean setting, falling back to def.
func (c *Context) Bool(key string, def bool) bool {
	v.SetDefault(c.key(key), def)

	return v.GetBool(c.key(key))
}

// Duration resolves a duration setting, falling back to def.
func (c *Context) Duration(key string, def time.Duration) time.Duration {
	v.SetDefault(c.key(key), def)

	return v.GetDuration(c.key(key))
}

// Slice- or struct-valued settings: add typed getters here as needed,
// following the same SetDefault-then-Get shape.
