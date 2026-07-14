package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

// BaseFlag is the type-erased view of a declared flag.
type BaseFlag interface {
	Name() string
	Description() string
	Type() string
	AddToSet(set *pflag.FlagSet)
}

// Flag is a declared flag whose resolved value has type T. Value resolves
// through Viper: flag → env var → YAML → flag default.
type Flag[T any] interface {
	BaseFlag
	Value() T
}

type infoFlag[T any] struct {
	name        string
	description string
	f           *pflag.Flag
	resolve     func(name string) T
}

func (in *infoFlag[T]) Name() string { return in.name }

func (in *infoFlag[T]) Description() string { return in.description }

func (in *infoFlag[T]) Value() T { return in.resolve(in.name) }

func (in *infoFlag[T]) Type() string {
	return fmt.Sprintf("%T", *new(T))
}

func (in *infoFlag[T]) AddToSet(set *pflag.FlagSet) { set.AddFlag(in.f) }

// newFlag wraps a flag just registered on the global registry, binding it to
// Viper and its env var.
func newFlag[T any](name, usage string, resolve func(string) T) *infoFlag[T] {
	f := &infoFlag[T]{
		name:        name,
		description: usage,
		f:           flags.Lookup(name),
		resolve:     resolve,
	}
	bindFlag(f.f)

	return f
}

// Bool declares a bool flag on the global registry and binds it to Viper.
func Bool(name string, def bool, usage string) Flag[bool] {
	flags.Bool(name, def, usage)

	return newFlag(name, usage, v.GetBool)
}

// String declares a string flag on the global registry and binds it to Viper.
func String(name, def, usage string) Flag[string] {
	flags.String(name, def, usage)

	return newFlag(name, usage, v.GetString)
}

// Int declares an int flag on the global registry and binds it to Viper.
func Int(name string, def int, usage string) Flag[int] {
	flags.Int(name, def, usage)

	return newFlag(name, usage, v.GetInt)
}

// Duration declares a duration flag on the global registry and binds it to
// Viper.
func Duration(name string, def time.Duration, usage string) Flag[time.Duration] {
	flags.Duration(name, def, usage)

	return newFlag(name, usage, v.GetDuration)
}

// bindFlag binds a flag to its Viper key and env var, and advertises the env
// var in the flag's usage text. Binding happens at declaration (package init),
// so a failure is a programming bug — hence panic.
func bindFlag(f *pflag.Flag) {
	env := EnvName(f.Name)

	if err := v.BindPFlag(f.Name, f); err != nil {
		panic(fmt.Sprintf("binding flag %s to viper: %v", f.Name, err))
	}

	if err := v.BindEnv(f.Name, env); err != nil {
		panic(fmt.Sprintf("binding flag %s to env %s: %v", f.Name, env, err))
	}

	f.Usage += " (env: " + env + ")"
}

// EnvName maps a flag name to its environment variable:
// "database.max-idle-conns" → "DATABASE_MAX_IDLE_CONNS".
func EnvName(flag string) string {
	return strings.ToUpper(envReplacer.Replace(flag))
}

var envReplacer = strings.NewReplacer("-", "_", ".", "_")
