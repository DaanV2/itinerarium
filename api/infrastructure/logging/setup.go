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

// LoggerConfigSet groups the logging flags. The set is declared here, next
// to the code that consumes it; the root command adds it to its persistent
// flags so every subcommand can tune logging.
var (
	LoggerConfigSet = config.New("log").WithValidate(validateLogger)

	LevelFlag = LoggerConfigSet.String("log.level", "info",
		"log level: debug, info, warn, error, fatal")
	FormatFlag = LoggerConfigSet.String("log.format", "text",
		"log format: text, json, logfmt")
	ReportCallerFlag = LoggerConfigSet.Bool("log.report-caller", false,
		"include the file:line that emitted each log entry")
)

func validateLogger(c *config.Config) error {
	_, err := log.ParseLevel(c.GetString("log.level"))

	return err
}

// Setup builds the global default logger from the "log" config set.
func Setup() {
	logger := log.NewWithOptions(os.Stderr, log.Options{
		TimeFormat:      time.DateTime,
		ReportTimestamp: true,
		Formatter:       log.TextFormatter,
	})

	apply(logger, ReportCallerFlag.Value(), LevelFlag.Value(), FormatFlag.Value())
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
