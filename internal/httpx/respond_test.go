package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		data       any
		wantStatus int
		wantJSON   string
		wantHeader string
	}{
		{
			name:       "simple struct",
			status:     http.StatusOK,
			data:       map[string]string{"message": "hello"},
			wantStatus: http.StatusOK,
			wantJSON:   `{"message":"hello"}`,
			wantHeader: "application/json",
		},
		{
			name:       "201 created",
			status:     http.StatusCreated,
			data:       map[string]int{"id": 123},
			wantStatus: http.StatusCreated,
			wantJSON:   `{"id":123}`,
			wantHeader: "application/json",
		},
		{
			name:   "nested struct",
			status: http.StatusOK,
			data: map[string]any{
				"user": map[string]any{
					"name":  "John",
					"email": "john@example.com",
				},
			},
			wantStatus: http.StatusOK,
			wantJSON:   `{"user":{"email":"john@example.com","name":"John"}}`,
			wantHeader: "application/json",
		},
		{
			name:       "empty object",
			status:     http.StatusOK,
			data:       map[string]string{},
			wantStatus: http.StatusOK,
			wantJSON:   `{}`,
			wantHeader: "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()

			WriteJSON(rr, tt.status, tt.data)

			if rr.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}

			if ct := rr.Header().Get("Content-Type"); ct != tt.wantHeader {
				t.Errorf("expected Content-Type %q, got %q", tt.wantHeader, ct)
			}

			// Normalize JSON for comparison (handles field ordering)
			var got, want any
			if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.wantJSON), &want); err != nil {
				t.Fatalf("failed to unmarshal expected JSON: %v", err)
			}

			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(want)

			if string(gotJSON) != string(wantJSON) {
				t.Errorf("expected JSON %s, got %s", wantJSON, gotJSON)
			}
		})
	}
}

func TestWriteError(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		code        string
		message     string
		details     any
		wantStatus  int
		wantError   string
		wantMessage string
		wantDetails any
	}{
		{
			name:        "simple error",
			status:      http.StatusBadRequest,
			code:        "invalid_input",
			message:     "name is required",
			details:     nil,
			wantStatus:  http.StatusBadRequest,
			wantError:   "invalid_input",
			wantMessage: "name is required",
			wantDetails: nil,
		},
		{
			name:        "error with details map",
			status:      http.StatusConflict,
			code:        "conflict",
			message:     "slug already exists",
			details:     map[string]string{"hint": "try a different slug"},
			wantStatus:  http.StatusConflict,
			wantError:   "conflict",
			wantMessage: "slug already exists",
			wantDetails: map[string]any{"hint": "try a different slug"},
		},
		{
			name:        "error with empty message",
			status:      http.StatusNotFound,
			code:        "not_found",
			message:     "",
			details:     nil,
			wantStatus:  http.StatusNotFound,
			wantError:   "not_found",
			wantMessage: "",
			wantDetails: nil,
		},
		{
			name:        "error with array details",
			status:      http.StatusBadRequest,
			code:        "validation_failed",
			message:     "multiple validation errors",
			details:     []string{"name too short", "email invalid"},
			wantStatus:  http.StatusBadRequest,
			wantError:   "validation_failed",
			wantMessage: "multiple validation errors",
			wantDetails: []any{"name too short", "email invalid"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()

			WriteError(rr, tt.status, tt.code, tt.message, tt.details)

			if rr.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}

			var response ErrorResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if response.Error != tt.wantError {
				t.Errorf("expected error %q, got %q", tt.wantError, response.Error)
			}

			if response.Message != tt.wantMessage {
				t.Errorf("expected message %q, got %q", tt.wantMessage, response.Message)
			}

			// Compare details as JSON to handle type conversions
			if tt.wantDetails != nil {
				gotJSON, _ := json.Marshal(response.Details)
				wantJSON, _ := json.Marshal(tt.wantDetails)
				if string(gotJSON) != string(wantJSON) {
					t.Errorf("expected details %s, got %s", wantJSON, gotJSON)
				}
			} else if response.Details != nil {
				t.Errorf("expected nil details, got %v", response.Details)
			}
		})
	}
}

func TestErrorResponse_JSONMarshaling(t *testing.T) {
	resp := ErrorResponse{
		Error:   "test_error",
		Message: "test message",
		Details: map[string]string{"key": "value"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled ErrorResponse
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.Error != resp.Error {
		t.Errorf("expected error %q, got %q", resp.Error, unmarshaled.Error)
	}
	if unmarshaled.Message != resp.Message {
		t.Errorf("expected message %q, got %q", resp.Message, unmarshaled.Message)
	}
}
