package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorResponse represents a JSON error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Details any    `json:"details,omitempty"`
}

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		// At this point headers are already sent, so we can't change the response
		// Just log the error
		slog.Error("failed to encode JSON response", "error", err)
	}
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, status int, code, message string, details any) {
	resp := ErrorResponse{
		Error:   code,
		Message: message,
		Details: details,
	}
	WriteJSON(w, status, resp)
}
