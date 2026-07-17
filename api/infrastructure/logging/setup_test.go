package logging_test

import (
	"bytes"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/logging"
	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/require"
)

func TestApplySetsLevel(t *testing.T) {
	var buf bytes.Buffer

	logger := log.New(&buf)
	logging.Apply(logger, false, "warn", "text")

	require.Equal(t, log.WarnLevel, logger.GetLevel())
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

			require.Contains(t, buf.String(), tt.want)
		})
	}
}

func TestSetupReadsLevelFromEnv(t *testing.T) {
	t.Setenv("LOG_LEVEL", "error")

	logging.Setup()

	require.Equal(t, log.ErrorLevel, log.Default().GetLevel())
}
