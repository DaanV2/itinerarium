package logging

import "github.com/charmbracelet/log"

// Apply is apply exported for external tests.
func Apply(logger *log.Logger, reportCaller bool, level, format string) {
	apply(logger, reportCaller, level, format)
}
