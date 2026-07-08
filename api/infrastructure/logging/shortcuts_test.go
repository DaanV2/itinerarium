package logging_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/logging"
	"github.com/charmbracelet/log"
)

func TestWithAttachesKeyValues(t *testing.T) {
	var buf bytes.Buffer

	logger := log.New(&buf)
	ctx := logging.Context(t.Context(), logger)

	logging.With(ctx, "request_id", "abc").Info("handled")

	if !strings.Contains(buf.String(), "request_id=abc") {
		t.Fatalf("expected output to contain the attached key-value, got %q", buf.String())
	}
}

func TestLevelShortcutsUseContextLogger(t *testing.T) {
	var buf bytes.Buffer

	logger := log.New(&buf)
	logger.SetLevel(log.DebugLevel)
	ctx := logging.Context(t.Context(), logger)

	logging.Debug(ctx, "debug msg")
	logging.Info(ctx, "info msg")
	logging.Warn(ctx, "warn msg")
	logging.Error(ctx, "error msg")

	out := buf.String()
	for _, want := range []string{"debug msg", "info msg", "warn msg", "error msg"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got %q", want, out)
		}
	}
}
