// Package logging configures the global charmbracelet/log logger and
// carries request-scoped loggers through context.Context (mechanus
// convention): [Context] stores a logger, [From] retrieves it and falls
// back to the global default when none was stored.
package logging

import (
	"os"
	"time"

	"github.com/DaanV2/itinerarium/api/infrastructure/config"
	"github.com/charmbracelet/log"
)

// Setup builds the global default logger from the "log" config context:
//
//	log.level          flag --level          env LOG_LEVEL          (debug|info|warn|error|fatal, default "info")
//	log.format         flag --format         env LOG_FORMAT         (text|json|logfmt, default "text")
//	log.report-caller  flag --report-caller  env LOG_REPORT_CALLER  (default false)
func Setup() {
	cfg := config.GetContext("log")

	logger := log.NewWithOptions(os.Stderr, log.Options{
		TimeFormat:      time.DateTime,
		ReportTimestamp: true,
		Formatter:       log.TextFormatter,
	})

	apply(logger, cfg.Bool("report-caller", false), cfg.String("level", "info"), cfg.String("format", "text"))
	log.SetDefault(logger)
}

func apply(logger *log.Logger, reportCaller bool, level, format string) {
	logger.SetReportCaller(reportCaller)

	parsed, err := log.ParseLevel(level)
	if err != nil {
		logger.Fatal("invalid log level", "level", level, "error", err)
	}

	logger.SetLevel(parsed)
	logger.SetFormatter(formatterFor(logger, format))
}

func formatterFor(logger *log.Logger, format string) log.Formatter {
	switch format {
	case "json":
		return log.JSONFormatter
	case "logfmt":
		return log.LogfmtFormatter
	case "text", "":
		return log.TextFormatter
	default:
		logger.Warn("unknown log format, falling back to text", "format", format)

		return log.TextFormatter
	}
}
