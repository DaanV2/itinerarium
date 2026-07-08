package logging

import (
	"context"

	"github.com/charmbracelet/log"
)

type contextKey struct{}

// From returns the logger stored in ctx by [Context], falling back to the
// global default logger when none was stored.
func From(ctx context.Context) *log.Logger {
	if ctx != nil {
		if logger, ok := ctx.Value(contextKey{}).(*log.Logger); ok {
			return logger
		}
	}

	return log.Default()
}

// FromPrefix returns the logger from ctx (see [From]) with prefix attached.
func FromPrefix(ctx context.Context, prefix string) *log.Logger {
	return From(ctx).WithPrefix(prefix)
}

// Context returns a copy of ctx carrying logger, retrievable via [From]. A
// nil logger is a no-op, returning ctx unchanged.
func Context(ctx context.Context, logger *log.Logger) context.Context {
	if logger == nil {
		return ctx
	}

	return context.WithValue(ctx, contextKey{}, logger)
}
