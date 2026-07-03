package application

import "errors"

// ErrNotFound is returned when an entity doesn't exist or the requester
// lacks access to it. Services use this — never ErrForbidden — for
// object-level checks, so existence never leaks through a 403.
var ErrNotFound = errors.New("not found")

// ErrForbidden is returned when an authenticated requester lacks the role
// required for an action (e.g. a player calling a GM-only admin endpoint).
// This is for action-level role gates, not object visibility — the action
// itself isn't a secret, the caller just isn't allowed to perform it.
var ErrForbidden = errors.New("forbidden")

// ErrUnauthenticated is returned when a request presents no valid
// credentials.
var ErrUnauthenticated = errors.New("unauthenticated")
