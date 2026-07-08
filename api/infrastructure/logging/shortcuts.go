package logging

import (
	"context"

	"github.com/charmbracelet/log"
)

// With returns the context logger (see [From]) with the given key-value
// pairs attached.
func With(ctx context.Context, keyvals ...any) *log.Logger {
	return From(ctx).With(keyvals...)
}

// Debug logs a debug-level message using the logger carried by ctx.
func Debug(ctx context.Context, msg any, keyvals ...any) {
	From(ctx).Debug(msg, keyvals...)
}

// Info logs an info-level message using the logger carried by ctx.
func Info(ctx context.Context, msg any, keyvals ...any) {
	From(ctx).Info(msg, keyvals...)
}

// Warn logs a warning-level message using the logger carried by ctx.
func Warn(ctx context.Context, msg any, keyvals ...any) {
	From(ctx).Warn(msg, keyvals...)
}

// Error logs an error-level message using the logger carried by ctx.
func Error(ctx context.Context, msg any, keyvals ...any) {
	From(ctx).Error(msg, keyvals...)
}
