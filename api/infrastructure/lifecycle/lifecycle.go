// Package lifecycle coordinates graceful shutdown across components
// (mechanus convention). A component opts into any phase by implementing the
// matching interface; ShutdownAll runs the phases in order over all
// components and joins the errors.
package lifecycle

import (
	"context"
	"errors"
)

// BeforeShutdown runs before anything is stopped (e.g. stop accepting jobs).
type BeforeShutdown interface {
	BeforeShutdown()
}

// Shutdown stops the component, bounded by the context deadline.
type Shutdown interface {
	Shutdown(ctx context.Context) error
}

// AfterShutDown runs after every component has stopped.
type AfterShutDown interface {
	AfterShutDown()
}

// ShutdownCleanup releases remaining resources (temp files, connections).
type ShutdownCleanup interface {
	ShutdownCleanup() error
}

// ShutdownAll drives all components through the four phases. Every phase runs
// for every component even when earlier ones fail; errors are joined.
func ShutdownAll(ctx context.Context, components ...any) error {
	runBefore(components)
	errs := runShutdown(ctx, components)
	runAfter(components)
	errs = append(errs, runCleanup(components)...)

	return errors.Join(errs...)
}

func runBefore(components []any) {
	for _, c := range components {
		if h, ok := c.(BeforeShutdown); ok {
			h.BeforeShutdown()
		}
	}
}

func runShutdown(ctx context.Context, components []any) []error {
	var errs []error

	for _, c := range components {
		if h, ok := c.(Shutdown); ok {
			if err := h.Shutdown(ctx); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errs
}

func runAfter(components []any) {
	for _, c := range components {
		if h, ok := c.(AfterShutDown); ok {
			h.AfterShutDown()
		}
	}
}

func runCleanup(components []any) []error {
	var errs []error

	for _, c := range components {
		if h, ok := c.(ShutdownCleanup); ok {
			if err := h.ShutdownCleanup(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errs
}
