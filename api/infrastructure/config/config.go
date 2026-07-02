// Package config wraps a Viper singleton. Resolution priority, highest first:
// command-line flags → environment variables → YAML config file → defaults.
//
// Every component reads its settings through a named [Context]:
//
//	cfg := config.GetContext("server")
//	addr := cfg.String("address", ":8080") // flag --address, env SERVER_ADDRESS, yaml server.address
package config

import (
	"strings"
	"sync"

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

// Load reads an optional YAML config file into the singleton. An empty path
// is a no-op (flags, env vars, and defaults still apply).
func Load(file string) error {
	if file == "" {
		return nil
	}
	v.SetConfigFile(file)

	return v.ReadInConfig()
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

// Duration-, slice-, or struct-valued settings: add typed getters here as
// needed, following the same SetDefault-then-Get shape.
