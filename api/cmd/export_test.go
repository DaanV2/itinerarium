package cmd

// RunInit exposes runInit so the external cmd_test package can exercise the
// init command's wiring without going through cobra's root execution (config
// auto-discovery, global flag state).
var RunInit = runInit
