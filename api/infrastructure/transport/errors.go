package transport

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
)

// serviceErrorStatus maps an application ErrorKind to its HTTP status. It is the
// single place transport translates the application layer's transport-agnostic
// error classification into status codes.
func serviceErrorStatus(kind application.ErrorKind) int {
	switch kind {
	case application.KindValidation:
		return http.StatusBadRequest
	case application.KindUnauthenticated:
		return http.StatusUnauthorized
	case application.KindForbidden:
		return http.StatusForbidden
	case application.KindNotFound:
		return http.StatusNotFound
	case application.KindConflict:
		return http.StatusConflict
	case application.KindInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// writeServiceError maps any application service error to its HTTP response.
// Service sentinels are *application.ServiceError values that carry their own
// ErrorKind and optional machine code, so this single function replaces the
// former per-entity write<Entity>ServiceError mappers. Anything that isn't a
// recognised ServiceError (or is tagged KindInternal) becomes a generic 500
// with no leaked detail.
func writeServiceError(w http.ResponseWriter, err error) {
	var se *application.ServiceError
	if !errors.As(err, &se) {
		xhttp.WriteError(w, http.StatusInternalServerError, fmt.Errorf("processing request: %w", err))

		return
	}

	status := serviceErrorStatus(se.Kind())
	if status == http.StatusInternalServerError {
		xhttp.WriteError(w, http.StatusInternalServerError, fmt.Errorf("processing request: %w", err))

		return
	}

	if code := se.Code(); code != "" {
		xhttp.WriteJSON(w, status, map[string]string{"error": err.Error(), "code": code})

		return
	}

	xhttp.WriteError(w, status, fmt.Errorf("processing request: %w", err))
}
