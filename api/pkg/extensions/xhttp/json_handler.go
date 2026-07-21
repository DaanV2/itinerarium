package xhttp

import (
	"net/http"
)

func JSONHandlerFunc(f func(w JSONResponseWriter, r *http.Request)) http.Handler {
	return JSONHandler{call: f}
}

type JSONResponseWriter struct {
	writer http.ResponseWriter
}

type JSONHandler struct {
	call func(w JSONResponseWriter, r *http.Request)
}

// ServeHTTP implements [http.Handler].
func (caller JSONHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	caller.call(JSONResponseWriter{writer: w}, r)
}

// WriteJSON encodes v as the JSON response body with the given status code.
func (jrw JSONResponseWriter) WriteJSON(status int, v any) {
	WriteJSON(jrw.writer, status, v)
}

// WriteError writes a {"error": message} JSON body with the given status code.
func (jrw JSONResponseWriter) WriteError(status int, err error) {
	WriteError(jrw.writer, status, err)
}

// WriteErrorMsg writes a {"error": message} JSON body with the given status code.
func (jrw JSONResponseWriter) WriteErrorMsg(status int, message string) {
	WriteErrorMsg(jrw.writer, status, message)
}

// WriteHeader sends the HTTP response header with the given status code.
func (jrw JSONResponseWriter) WriteHeader(status int) {
	jrw.writer.WriteHeader(status)
}
