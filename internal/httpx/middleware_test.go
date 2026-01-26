package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestGetRequestID(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{
			name: "request ID exists",
			ctx:  context.WithValue(context.Background(), requestIDContextKey, "test-123"),
			want: "test-123",
		},
		{
			name: "request ID missing",
			ctx:  context.Background(),
			want: "",
		},
		{
			name: "wrong type in context",
			ctx:  context.WithValue(context.Background(), requestIDContextKey, 12345),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetRequestID(tt.ctx)
			if got != tt.want {
				t.Errorf("GetRequestID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	requestID := "test-request-id"

	newCtx := WithRequestID(ctx, requestID)

	got := GetRequestID(newCtx)
	if got != requestID {
		t.Errorf("expected request ID %q, got %q", requestID, got)
	}

	// Verify original context is unchanged
	if GetRequestID(ctx) != "" {
		t.Error("original context should not have request ID")
	}
}

func TestChain(t *testing.T) {
	// Track middleware execution order
	var calls []string

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls = append(calls, "m1-before")
			next.ServeHTTP(w, r)
			calls = append(calls, "m1-after")
		})
	}

	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls = append(calls, "m2-before")
			next.ServeHTTP(w, r)
			calls = append(calls, "m2-after")
		})
	}

	m3 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls = append(calls, "m3-before")
			next.ServeHTTP(w, r)
			calls = append(calls, "m3-after")
		})
	}

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, "handler")
	})

	// Chain middleware
	chained := Chain(m1, m2, m3)(finalHandler)

	// Execute
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	chained.ServeHTTP(rr, req)

	// Verify execution order: m1 -> m2 -> m3 -> handler -> m3 -> m2 -> m1
	expected := []string{
		"m1-before",
		"m2-before",
		"m3-before",
		"handler",
		"m3-after",
		"m2-after",
		"m1-after",
	}

	if len(calls) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(calls), calls)
	}

	for i, want := range expected {
		if calls[i] != want {
			t.Errorf("call %d: expected %q, got %q", i, want, calls[i])
		}
	}
}

func TestChain_Empty(t *testing.T) {
	called := false
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	// Chain with no middleware
	chained := Chain()(finalHandler)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	chained.ServeHTTP(rr, req)

	if !called {
		t.Error("handler should be called even with no middleware")
	}
}

func TestChain_Single(t *testing.T) {
	var calls []string

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls = append(calls, "m1")
			next.ServeHTTP(w, r)
		})
	}

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, "handler")
	})

	chained := Chain(m1)(finalHandler)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	chained.ServeHTTP(rr, req)

	expected := []string{"m1", "handler"}
	if len(calls) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, calls)
	}
}

func TestRequestID_GeneratesUUID(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r.Context())
		if requestID == "" {
			t.Error("expected request ID to be set in context")
		}

		// Verify it's a valid UUID
		if _, err := uuid.Parse(requestID); err != nil {
			t.Errorf("expected valid UUID, got %q: %v", requestID, err)
		}

		// Verify it's set in response header
		headerID := w.Header().Get(RequestIDHeader)
		if headerID != requestID {
			t.Errorf("expected header %q, got %q", requestID, headerID)
		}
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
}

func TestRequestID_UsesExistingHeader(t *testing.T) {
	existingID := "existing-request-id-123"

	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r.Context())
		if requestID != existingID {
			t.Errorf("expected request ID %q, got %q", existingID, requestID)
		}

		headerID := w.Header().Get(RequestIDHeader)
		if headerID != existingID {
			t.Errorf("expected header %q, got %q", existingID, headerID)
		}
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(RequestIDHeader, existingID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
}

func TestCORS_AllowAllOrigins(t *testing.T) {
	handler := CORS(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("expected Access-Control-Allow-Origin '*', got %q", got)
	}
}

func TestCORS_AllowedOrigins(t *testing.T) {
	allowedOrigins := []string{"https://example.com", "https://app.example.com"}
	handler := CORS(allowedOrigins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name        string
		origin      string
		wantAllowed bool
	}{
		{
			name:        "allowed origin",
			origin:      "https://example.com",
			wantAllowed: true,
		},
		{
			name:        "another allowed origin",
			origin:      "https://app.example.com",
			wantAllowed: true,
		},
		{
			name:        "disallowed origin",
			origin:      "https://evil.com",
			wantAllowed: false,
		},
		{
			name:        "no origin header",
			origin:      "",
			wantAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			allowOrigin := rr.Header().Get("Access-Control-Allow-Origin")
			if tt.wantAllowed {
				if allowOrigin != tt.origin {
					t.Errorf("expected Access-Control-Allow-Origin %q, got %q", tt.origin, allowOrigin)
				}
			} else {
				if allowOrigin != "" {
					t.Errorf("expected no Access-Control-Allow-Origin, got %q", allowOrigin)
				}
			}
		})
	}
}

func TestCORS_PreflightRequest(t *testing.T) {
	handlerCalled := false
	handler := CORS(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	}))

	req := httptest.NewRequest("OPTIONS", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if handlerCalled {
		t.Error("handler should not be called for OPTIONS preflight")
	}

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, rr.Code)
	}

	// Verify CORS headers are set
	if got := rr.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("expected Access-Control-Allow-Methods to be set")
	}
	if got := rr.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("expected Access-Control-Allow-Headers to be set")
	}
}

func TestResponseWriter_CapturesStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"201 Created", http.StatusCreated},
		{"400 Bad Request", http.StatusBadRequest},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			wrapped := &responseWriter{ResponseWriter: rr, statusCode: http.StatusOK}

			wrapped.WriteHeader(tt.statusCode)

			if wrapped.statusCode != tt.statusCode {
				t.Errorf("expected status code %d, got %d", tt.statusCode, wrapped.statusCode)
			}
		})
	}
}

func TestResponseWriter_DefaultStatusCode(t *testing.T) {
	rr := httptest.NewRecorder()
	wrapped := &responseWriter{ResponseWriter: rr, statusCode: http.StatusOK}

	// Don't call WriteHeader - should keep default
	if wrapped.statusCode != http.StatusOK {
		t.Errorf("expected default status code %d, got %d", http.StatusOK, wrapped.statusCode)
	}
}
