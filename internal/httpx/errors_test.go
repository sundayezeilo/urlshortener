package httpx

import (
	"net/http"
	"testing"

	"github.com/sundayezeilo/urlshortener/internal/errx"
)

func TestErrorKindToStatus(t *testing.T) {
	tests := []struct {
		name       string
		kind       errx.Kind
		wantStatus int
	}{
		{
			name:       "not found",
			kind:       errx.NotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "conflict",
			kind:       errx.Conflict,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "invalid",
			kind:       errx.Invalid,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unauthorized",
			kind:       errx.Unauthorized,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "forbidden",
			kind:       errx.Forbidden,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "unavailable",
			kind:       errx.Unavailable,
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "internal",
			kind:       errx.Internal,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "unknown",
			kind:       errx.Unknown,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "invalid kind value",
			kind:       errx.Kind(99),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ErrorKindToStatus(tt.kind)
			if got != tt.wantStatus {
				t.Errorf("ErrorKindToStatus(%v) = %d, want %d", tt.kind, got, tt.wantStatus)
			}
		})
	}
}

func TestErrorKindToCode(t *testing.T) {
	tests := []struct {
		name     string
		kind     errx.Kind
		wantCode string
	}{
		{
			name:     "not found",
			kind:     errx.NotFound,
			wantCode: "not_found",
		},
		{
			name:     "conflict",
			kind:     errx.Conflict,
			wantCode: "conflict",
		},
		{
			name:     "invalid",
			kind:     errx.Invalid,
			wantCode: "invalid_input",
		},
		{
			name:     "unauthorized",
			kind:     errx.Unauthorized,
			wantCode: "unauthorized",
		},
		{
			name:     "forbidden",
			kind:     errx.Forbidden,
			wantCode: "forbidden",
		},
		{
			name:     "unavailable",
			kind:     errx.Unavailable,
			wantCode: "unavailable",
		},
		{
			name:     "internal",
			kind:     errx.Internal,
			wantCode: "internal_error",
		},
		{
			name:     "unknown",
			kind:     errx.Unknown,
			wantCode: "internal_error",
		},
		{
			name:     "invalid kind value",
			kind:     errx.Kind(99),
			wantCode: "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ErrorKindToCode(tt.kind)
			if got != tt.wantCode {
				t.Errorf("ErrorKindToCode(%v) = %q, want %q", tt.kind, got, tt.wantCode)
			}
		})
	}
}

func TestErrorKindMappingConsistency(t *testing.T) {
	// Verify that all error kinds are handled consistently
	tests := []struct {
		name string
		kind errx.Kind
	}{
		{"NotFound", errx.NotFound},
		{"Conflict", errx.Conflict},
		{"Invalid", errx.Invalid},
		{"Unauthorized", errx.Unauthorized},
		{"Forbidden", errx.Forbidden},
		{"Unavailable", errx.Unavailable},
		{"Internal", errx.Internal},
		{"Unknown", errx.Unknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := ErrorKindToStatus(tt.kind)
			code := ErrorKindToCode(tt.kind)

			// Verify we got non-zero values
			if status == 0 {
				t.Error("ErrorKindToStatus returned 0")
			}
			if code == "" {
				t.Error("ErrorKindToCode returned empty string")
			}

			// Verify status is a valid HTTP status code
			if status < 100 || status >= 600 {
				t.Errorf("invalid HTTP status code: %d", status)
			}
		})
	}
}
