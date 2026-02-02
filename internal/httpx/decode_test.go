package httpx

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

type testRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

func TestDecodeJSON(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		contentType string
		wantErr     bool
		errContains string
		validate    func(*testing.T, testRequest)
	}{
		{
			name:        "valid JSON",
			body:        `{"name":"John","email":"john@example.com","age":30}`,
			contentType: "application/json",
			wantErr:     false,
			validate: func(t *testing.T, req testRequest) {
				if req.Name != "John" {
					t.Errorf("expected name 'John', got %q", req.Name)
				}
				if req.Email != "john@example.com" {
					t.Errorf("expected email 'john@example.com', got %q", req.Email)
				}
				if req.Age != 30 {
					t.Errorf("expected age 30, got %d", req.Age)
				}
			},
		},
		{
			name:        "empty body",
			body:        "",
			contentType: "application/json",
			wantErr:     true,
			errContains: "request body is empty",
		},
		{
			name:        "malformed JSON - missing quote",
			body:        `{"name":"John,"email":"john@example.com"}`,
			contentType: "application/json",
			wantErr:     true,
			errContains: "malformed JSON",
		},
		{
			name:        "malformed JSON - trailing comma",
			body:        `{"name":"John","email":"john@example.com",}`,
			contentType: "application/json",
			wantErr:     true,
			errContains: "malformed JSON",
		},
		{
			name:        "unknown field",
			body:        `{"name":"John","email":"john@example.com","unknown":"field"}`,
			contentType: "application/json",
			wantErr:     true,
			errContains: "unknown",
		},
		{
			name:        "invalid type for field",
			body:        `{"name":"John","email":"john@example.com","age":"thirty"}`,
			contentType: "application/json",
			wantErr:     true,
			errContains: "invalid value for field",
		},
		{
			name:        "multiple JSON objects",
			body:        `{"name":"John","email":"john@example.com"}{"name":"Jane"}`,
			contentType: "application/json",
			wantErr:     true,
			errContains: "multiple JSON objects",
		},
		{
			name:        "body too large",
			body:        `{"name":"` + strings.Repeat("x", MaxRequestBodySize+1) + `"}`,
			contentType: "application/json",
			wantErr:     true,
			errContains: "request body too large",
		},
		{
			name:        "partial JSON - can decode but more data exists",
			body:        `{"name":"John","email":"john@example.com","age":30}extra`,
			contentType: "application/json",
			wantErr:     true,
			errContains: "multiple JSON objects",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			result, err := DecodeJSON[testRequest](req)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestDecodeJSON_ZeroValueOnError(t *testing.T) {
	req := httptest.NewRequest("POST", "/test", strings.NewReader("invalid json"))

	result, err := DecodeJSON[testRequest](req)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify zero value is returned
	var zero testRequest
	if result != zero {
		t.Errorf("expected zero value on error, got %+v", result)
	}
}

func TestDecodeJSON_ClosesBody(t *testing.T) {
	body := &testReadCloser{
		Reader: strings.NewReader(`{"name":"John","email":"test@example.com","age":25}`),
		closed: false,
	}

	req := httptest.NewRequest("POST", "/test", body)

	_, err := DecodeJSON[testRequest](req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !body.closed {
		t.Error("expected body to be closed")
	}
}

// testReadCloser helps verify that body is closed
type testReadCloser struct {
	io.Reader
	closed bool
}

func (t *testReadCloser) Close() error {
	t.closed = true
	return nil
}
