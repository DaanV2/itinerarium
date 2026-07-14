package config

import (
	"errors"
	"fmt"
	"sync"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	// flags is the global flag registry. It is never parsed itself — commands
	// borrow individual flag instances from it via Config.AddToSet, so a flag
	// shared by two commands is one definition with one binding.
	flags = pflag.NewFlagSet("global", pflag.ContinueOnError)

	// v is the Viper instance every flag resolves through.
	v = newViper()

	// registry holds every named config set, for Get and Validate.
	registry sync.Map // set name → *Config
)

func newViper() *viper.Viper {
	nv := viper.New()
	nv.SetEnvKeyReplacer(envReplacer)
	nv.AutomaticEnv()

	return nv
}

// New creates (and registers) a named config set for one component.
func New(name string) *Config {
	c := &Config{
		name: name,
		data: map[string]BaseFlag{},
	}

	registry.Store(name, c)

	return c
}

// Get returns a previously created config set by name; a missing set is a
// programming bug.
func Get(name string) *Config {
	item, ok := registry.Load(name)
	if !ok {
		panic("no such config set exists: " + name)
	}

	return item.(*Config)
}

// All returns every registered config set.
func All() []*Config {
	result := make([]*Config, 0)

	registry.Range(func(_, value any) bool {
		if c, ok := value.(*Config); ok {
			result = append(result, c)
		}

		return true
	})

	return result
}

// Validate checks every registered config set against its validator and
// joins the failures. Call it once the config file is loaded, before any
// component reads its settings.
func Validate() error {
	var err error
	for _, c := range All() {
		if verr := c.Validate(); verr != nil {
			err = errors.Join(err, fmt.Errorf("config %s: %w", c.Name(), verr))
		}
	}

	return err
}
