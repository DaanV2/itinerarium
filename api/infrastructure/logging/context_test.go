package logging_test

import (
	"bytes"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/logging"
	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/require"
)

func TestFromFallsBackToDefault(t *testing.T) {
	require.Same(t, log.Default(), logging.From(t.Context()), "From should fall back to the global default logger")
}

func TestFromReturnsStoredLogger(t *testing.T) {
	var buf bytes.Buffer

	logger := log.New(&buf)
	ctx := logging.Context(t.Context(), logger)

	require.Same(t, logger, logging.From(ctx), "From should return the logger stored via Context")
}

func TestContextWithNilLoggerIsNoop(t *testing.T) {
	ctx := t.Context()

	require.Equal(t, ctx, logging.Context(ctx, nil), "Context with a nil logger should return ctx unchanged")
}

func TestFromPrefixAttachesPrefix(t *testing.T) {
	var buf bytes.Buffer

	logger := log.New(&buf)
	ctx := logging.Context(t.Context(), logger)

	logging.FromPrefix(ctx, "db").Info("opening")

	require.Contains(t, buf.String(), "db:")
}
