// Package config implements flag-backed configuration sets (mechanus
// convention). The component that consumes a setting declares it as a typed
// flag on a named [Config] set at package init; commands opt in with
// [Config.AddToSet]; values resolve through Viper with priority: command-line
// flags → environment variables → YAML config file → flag defaults.
//
//	var (
//		ServerConfigSet = config.New("server")
//		AddressFlag     = ServerConfigSet.String("server.address", ":8080", "listen address")
//	)
//
//	addr := AddressFlag.Value()
//
// The flag name doubles as every other key: flag --server.address, env var
// SERVER_ADDRESS, and nested YAML key server.address. Because every flag
// lives in one global registry, two commands adding the same set share the
// same flag instances — no duplicate definitions, no re-binding.
//
// If no explicit file is passed to [Load], a config.yaml is searched for in
// the directories returned by [ConfigPaths].
package config

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/pflag"
)

// Config is a named set of flags that belong to one component. Declare flags
// on it at package init, hand them to commands with AddToSet, and validate
// the resolved values with Validate.
type Config struct {
	name       string
	mu         sync.RWMutex
	data       map[string]BaseFlag
	validateFn func(*Config) error
}

// AddToSet registers every flag in this set on a command's flag set.
func (c *Config) AddToSet(set *pflag.FlagSet) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, f := range c.data {
		f.AddToSet(set)
	}
}

// Name returns the component name this set was created with.
func (c *Config) Name() string { return c.name }

// Load looks up a declared flag by name.
func (c *Config) Load(name string) (BaseFlag, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	f, ok := c.data[name]
	if !ok {
		return nil, errors.New("couldn't find flag " + name + " in config set " + c.name)
	}

	return f, nil
}

// MustLoad is Load for lookups where a missing flag is a bug.
func (c *Config) MustLoad(name string) BaseFlag {
	f, err := c.Load(name)
	if err != nil {
		panic(err)
	}

	return f
}

func (c *Config) store(name string, f BaseFlag) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[name] = f
}

// Bool declares a bool flag on this set.
func (c *Config) Bool(name string, def bool, usage string) Flag[bool] {
	f := Bool(name, def, usage)
	c.store(name, f)

	return f
}

// GetBool resolves a bool flag declared on this set.
func (c *Config) GetBool(name string) bool {
	return getValue[bool](c, name)
}

// String declares a string flag on this set.
func (c *Config) String(name, def, usage string) Flag[string] {
	f := String(name, def, usage)
	c.store(name, f)

	return f
}

// GetString resolves a string flag declared on this set.
func (c *Config) GetString(name string) string {
	return getValue[string](c, name)
}

// Int declares an int flag on this set.
func (c *Config) Int(name string, def int, usage string) Flag[int] {
	f := Int(name, def, usage)
	c.store(name, f)

	return f
}

// GetInt resolves an int flag declared on this set.
func (c *Config) GetInt(name string) int {
	return getValue[int](c, name)
}

// Duration declares a duration flag on this set.
func (c *Config) Duration(name string, def time.Duration, usage string) Flag[time.Duration] {
	f := Duration(name, def, usage)
	c.store(name, f)

	return f
}

// GetDuration resolves a duration flag declared on this set.
func (c *Config) GetDuration(name string) time.Duration {
	return getValue[time.Duration](c, name)
}

// WithValidate couples fn as the validator for this set (see Validate).
// A nil fn means the set is always valid.
func (c *Config) WithValidate(fn func(*Config) error) *Config {
	c.validateFn = fn

	return c
}

// Validate checks the resolved values of this set with the function given to
// WithValidate.
func (c *Config) Validate() error {
	if c.validateFn == nil {
		return nil
	}

	return c.validateFn(c)
}

func getValue[T any](c *Config, name string) T {
	f := c.MustLoad(name)

	v, ok := f.(Flag[T])
	if !ok {
		panic(fmt.Sprintf("flag %s in config set %s is not a %T but a %s", name, c.name, *new(T), f.Type()))
	}

	return v.Value()
}
