// Package components is the composition root (dependency-injection container).
// It wires config, database, authentication, repositories, application
// services, and transport into ready-to-use components so the Cobra commands
// in cmd/ stay thin: build the pieces here, run them there.
//
// The builders are layered smallest-to-largest — SetupDatabase,
// NewRepositories, SetupAuthentication, NewServices, CreateRouter — and
// BuildServer composes them into a ServerComponents. Commands that only need a
// subset (e.g. init reuses SetupDatabase + NewServices) call the lower-level
// builders directly.
package components
