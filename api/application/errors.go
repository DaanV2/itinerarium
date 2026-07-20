package application

import "errors"

// ErrorKind classifies a service error so the transport layer can map it to a
// response status without a per-entity switch. It is deliberately transport-
// agnostic: the application layer names the *kind* of failure, and transport
// owns the concrete HTTP mapping (see transport.writeServiceError).
type ErrorKind int

const (
	// KindInternal is an unexpected failure. Transport maps it to 500 and hides
	// the detail. It is the zero value, so any plain error reaching transport is
	// treated as internal.
	KindInternal ErrorKind = iota
	// KindValidation is a malformed or semantically invalid request → 400.
	KindValidation
	// KindUnauthenticated means no valid credentials were presented → 401.
	KindUnauthenticated
	// KindForbidden means the caller is known but lacks the required role → 403.
	// This is for action-level role gates, not object visibility — the action
	// itself isn't a secret, the caller just isn't allowed to perform it.
	KindForbidden
	// KindNotFound means the entity doesn't exist or is hidden from the caller
	// → 404. Object-level access failures use this, never KindForbidden, so
	// existence never leaks through a 403.
	KindNotFound
	// KindConflict means the request conflicts with current state → 409.
	KindConflict
)

// ServiceError is an error that carries its ErrorKind and an optional machine
// code (e.g. "path_collision"). Service sentinels are ServiceErrors, so a
// single transport writeServiceError can map any of them to the right status
// without knowing which entity produced it.
type ServiceError struct {
	kind ErrorKind
	code string
	err  error
}

// Error implements error, delegating to the wrapped message.
func (e *ServiceError) Error() string { return e.err.Error() }

// Unwrap exposes the wrapped error so errors.Is/errors.As traverse into it.
// This is what lets a reclassified sentinel (see withKind) still match the
// original sentinel under errors.Is.
func (e *ServiceError) Unwrap() error { return e.err }

// Kind reports how transport should classify the error.
func (e *ServiceError) Kind() ErrorKind { return e.kind }

// Code reports the optional machine code, or "" when none is set.
func (e *ServiceError) Code() string { return e.code }

// serviceErr builds a sentinel ServiceError with a fixed message.
func serviceErr(kind ErrorKind, msg string) *ServiceError {
	return &ServiceError{kind: kind, err: errors.New(msg)}
}

// codedServiceErr builds a sentinel ServiceError that also carries a machine
// code, mirroring what path_collision / concurrent_edit expose to clients.
func codedServiceErr(kind ErrorKind, code, msg string) *ServiceError {
	return &ServiceError{kind: kind, code: code, err: errors.New(msg)}
}

// withKind reclassifies err under a different kind while preserving its message
// and identity — errors.Is against the original sentinel still matches. It is
// for the rare case where one sentinel means different things at different call
// sites: an unknown currency is a 400 (KindValidation) when referenced in an
// inventory write but a 404 (KindNotFound) when looked up directly in the
// catalog.
func withKind(kind ErrorKind, err error) *ServiceError {
	return &ServiceError{kind: kind, err: err}
}

// ErrNotFound is returned when an entity doesn't exist or the requester
// lacks access to it. Services use this — never ErrForbidden — for
// object-level checks, so existence never leaks through a 403.
var ErrNotFound = serviceErr(KindNotFound, "not found")

// ErrForbidden is returned when an authenticated requester lacks the role
// required for an action (e.g. a player calling a GM-only admin endpoint).
// This is for action-level role gates, not object visibility — the action
// itself isn't a secret, the caller just isn't allowed to perform it.
var ErrForbidden = serviceErr(KindForbidden, "forbidden")

// ErrUnauthenticated is returned when a request presents no valid
// credentials.
var ErrUnauthenticated = serviceErr(KindUnauthenticated, "unauthenticated")
