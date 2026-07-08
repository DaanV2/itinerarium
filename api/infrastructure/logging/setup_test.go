package logging_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/logging"
	"github.com/charmbracelet/log"
)

func TestApplySetsLevel(t *testing.T) {
	var buf bytes.Buffer

	logger := log.New(&buf)
	logging.Apply(logger, false, "warn", "text")

	if logger.GetLevel() != log.WarnLevel {
		t.Fatalf("expected warn level, got %s", logger.GetLevel())
	}
}

func TestApplyFormats(t *testing.T) {
	tests := map[string]struct {
		format string
		want   string
	}{
		"json":    {format: "json", want: `"msg":"hello"`},
		"logfmt":  {format: "logfmt", want: "msg=hello"},
		"text":    {format: "text", want: "hello"},
		"unknown": {format: "bogus", want: "hello"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer

			logger := log.New(&buf)
			logging.Apply(logger, false, "info", tt.format)
			logger.Info("hello")

			if !strings.Contains(buf.String(), tt.want) {
				t.Fatalf("expected output to contain %q, got %q", tt.want, buf.String())
			}
		})
	}
}

func TestSetupReadsLevelFromEnv(t *testing.T) {
	t.Setenv("LOG_LEVEL", "error")

	logging.Setup()

	if log.Default().GetLevel() != log.ErrorLevel {
		t.Fatalf("expected error level, got %s", log.Default().GetLevel())
	}
}
