package httpx

import (
	"net/http"

	"github.com/sundayezeilo/urlshortener/internal/errx"
)

// ErrorKindToStatus maps errx.Kind to HTTP status codes.
// Handlers can use this as a helper when mapping their own errors.
func ErrorKindToStatus(kind errx.Kind) int {
	switch kind {
	case errx.NotFound:
		return http.StatusNotFound
	case errx.Conflict:
		return http.StatusConflict
	case errx.Invalid:
		return http.StatusBadRequest
	case errx.Unauthorized:
		return http.StatusUnauthorized
	case errx.Forbidden:
		return http.StatusForbidden
	case errx.Unavailable:
		return http.StatusServiceUnavailable
	case errx.Internal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// ErrorKindToCode maps errx.Kind to error codes for JSON responses.
// Handlers can use this as a helper when mapping their own errors.
func ErrorKindToCode(kind errx.Kind) string {
	switch kind {
	case errx.NotFound:
		return "not_found"
	case errx.Conflict:
		return "conflict"
	case errx.Invalid:
		return "invalid_input"
	case errx.Unauthorized:
		return "unauthorized"
	case errx.Forbidden:
		return "forbidden"
	case errx.Unavailable:
		return "unavailable"
	case errx.Internal:
		return "internal_error"
	default:
		return "internal_error"
	}
}
