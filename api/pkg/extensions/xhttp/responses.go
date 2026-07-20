package xhttp

import (
	"encoding/json"
	"net/http"

	"github.com/DaanV2/itinerarium/api/infrastructure/logging"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

// WriteJSON encodes v as the JSON response body with the given status code.
func WriteJSON[T any](w http.ResponseWriter, status int, v T) {
	body, err := json.Marshal(v)
	if err != nil {
		logging.Default().Error("error encoding response", err)
		http.Error(w, "encoding response", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(body)
	if err != nil {
		logging.Default().Error("error writing response", err)
	}
}

// WriteError writes a {"error": message} JSON body with the given status code.
func WriteError(w http.ResponseWriter, status int, err error) {
	WriteJSON(w, status, ErrorResponse{err.Error()})
}

// WriteErrorMsg writes a {"error": message} JSON body with the given status code.
func WriteErrorMsg(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{message})
}
