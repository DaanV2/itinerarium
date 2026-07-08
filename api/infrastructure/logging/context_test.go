package logging_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/logging"
	"github.com/charmbracelet/log"
)

func TestFromFallsBackToDefault(t *testing.T) {
	if logging.From(t.Context()) != log.Default() {
		t.Fatal("expected From to fall back to the global default logger")
	}
}

func TestFromReturnsStoredLogger(t *testing.T) {
	var buf bytes.Buffer

	logger := log.New(&buf)
	ctx := logging.Context(t.Context(), logger)

	if logging.From(ctx) != logger {
		t.Fatal("expected From to return the logger stored via Context")
	}
}

func TestContextWithNilLoggerIsNoop(t *testing.T) {
	ctx := t.Context()

	if logging.Context(ctx, nil) != ctx {
		t.Fatal("expected Context with a nil logger to return ctx unchanged")
	}
}

func TestFromPrefixAttachesPrefix(t *testing.T) {
	var buf bytes.Buffer

	logger := log.New(&buf)
	ctx := logging.Context(t.Context(), logger)

	logging.FromPrefix(ctx, "db").Info("opening")

	if !strings.Contains(buf.String(), "db:") {
		t.Fatalf("expected output to contain the prefix, got %q", buf.String())
	}
}
