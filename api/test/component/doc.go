// Package component holds full-server ("component") tests: black-box tests that
// boot the entire application through the real composition root
// (components.BuildServer) against an in-memory database and drive it over real
// HTTP, exactly as a client would.
//
// Use these to prove that the assembled system — routing, middleware, auth,
// services, and persistence wired together — behaves correctly end to end. Use
// the per-package _test.go files next to the code for unit and layer-level
// coverage; reach for a component test when the behaviour under test only
// emerges once every layer is wired together.
//
// The Harness type (harness.go) builds one isolated server per test and exposes
// helpers for issuing requests and authenticating. Tests live in the external
// component_test package and depend on Harness.
package component
