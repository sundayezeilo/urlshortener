package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"slices"
	"time"

	"github.com/google/uuid"
)

const (
	// RequestIDHeader is the header name for request ID.
	RequestIDHeader = "X-Request-ID"
)

// contextKey is the type for context keys to avoid collisions.
type contextKey string

const requestIDContextKey contextKey = "request_id"

// Middleware represents a function that wraps an http.Handler.
type Middleware func(http.Handler) http.Handler

// Chain applies multiple middleware in order.
// Example: Chain(middleware1, middleware2, middleware3)(handler)
func Chain(middlewares ...Middleware) Middleware {
	return func(final http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}

// RequestID is a middleware that adds a unique request ID to each request.
// It first checks for an existing X-Request-ID header, and generates one if not present.
// The request ID is added to the request context and also set as a response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(RequestIDHeader)

		if requestID == "" {
			requestID = uuid.New().String()
		}

		w.Header().Set(RequestIDHeader, requestID)

		ctx := context.WithValue(r.Context(), requestIDContextKey, requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID extracts the request ID from context.
// Returns empty string if not found.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDContextKey).(string); ok {
		return id
	}
	return ""
}

// WithRequestID adds a request ID to the context.
// This is useful for testing or manually setting request IDs.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey, requestID)
}

// Logger is a middleware that logs HTTP requests with structured logging.
func Logger(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(wrapped, r)
			duration := time.Since(start)

			logger.InfoContext(r.Context(), "http request",
				"request_id", GetRequestID(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
				// "user_agent", r.UserAgent(),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}

// Recovery is a middleware that recovers from panics and returns a 500 error.
func Recovery(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Log the panic with stack trace
					logger.ErrorContext(r.Context(), "panic recovered",
						"request_id", GetRequestID(r.Context()),
						"error", err,
						"stack", string(debug.Stack()),
					)

					// Return 500 error
					WriteError(w, http.StatusInternalServerError,
						"internal_error",
						"an unexpected error occurred",
						nil)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// CORS is a middleware that adds CORS headers.
// For production, allowed origins should configure more carefully.
func CORS(allowedOrigins []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Simple implementation - allow all if no origins specified
			if len(allowedOrigins) == 0 {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				if slices.Contains(allowedOrigins, origin) {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "86400") // 86400 seconds = 24 hours = 1 day

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
