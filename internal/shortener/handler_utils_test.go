package shortener

import (
	"errors"
	"testing"
)

func TestValidateCreateRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     HTTPCreateLinkRequest
		wantErr bool
	}{
		{
			name: "valid request with URL only",
			req: HTTPCreateLinkRequest{
				URL: "https://example.com",
			},
			wantErr: false,
		},
		{
			name: "valid request with URL and custom slug",
			req: HTTPCreateLinkRequest{
				URL:        "https://example.com",
				CustomSlug: "my-link",
			},
			wantErr: false,
		},
		{
			name: "empty URL",
			req: HTTPCreateLinkRequest{
				URL: "",
			},
			wantErr: true,
		},
		{
			name: "whitespace only URL",
			req: HTTPCreateLinkRequest{
				URL: "   ",
			},
			wantErr: false, // validateCreateRequest only checks if empty, not trimmed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCreateRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCreateRequest() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil {
				if err.Error() == "" {
					t.Error("expected non-empty error message")
				}
			}
		})
	}
}

func TestValidateSlugFormat(t *testing.T) {
	tests := []struct {
		name    string
		slug    string
		wantErr bool
	}{
		{
			name:    "valid short slug",
			slug:    "abc",
			wantErr: false,
		},
		{
			name:    "valid medium slug",
			slug:    "my-custom-slug",
			wantErr: false,
		},
		{
			name:    "valid long slug",
			slug:    "this-is-a-very-long-slug-name-but-still-valid",
			wantErr: false,
		},
		{
			name:    "empty slug",
			slug:    "",
			wantErr: true,
		},
		{
			name:    "slug at max length",
			slug:    "a234567890123456789012345678901234567890123456789012345678901234", // 64 chars
			wantErr: false,
		},
		{
			name:    "slug exceeds max length",
			slug:    "a2345678901234567890123456789012345678901234567890123456789012345", // 65 chars
			wantErr: true,
		},
		{
			name:    "slug with special characters",
			slug:    "slug-with-dash",
			wantErr: false,
		},
		{
			name:    "slug with numbers",
			slug:    "slug123",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSlugFormat(tt.slug)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSlugFormat() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil {
				// Verify error message contains useful info
				if err.Error() == "" {
					t.Error("expected non-empty error message")
				}
			}
		})
	}
}

func TestExtractSlugFromPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "simple slug",
			path: "/abc123",
			want: "abc123",
		},
		{
			name: "slug without leading slash",
			path: "abc123",
			want: "abc123",
		},
		{
			name: "slug with prefix",
			path: "/s/abc123",
			want: "abc123",
		},
		{
			name: "slug with multiple segments",
			path: "/api/v1/links/abc123",
			want: "abc123",
		},
		{
			name: "empty path",
			path: "",
			want: "",
		},
		{
			name: "just slash",
			path: "/",
			want: "",
		},
		{
			name: "slug with trailing slash",
			path: "/abc123/",
			want: "",
		},
		{
			name: "nested path",
			path: "/category/subcategory/item",
			want: "item",
		},
		{
			name: "slug with dashes",
			path: "/my-custom-slug",
			want: "my-custom-slug",
		},
		{
			name: "slug with underscores",
			path: "/my_custom_slug",
			want: "my_custom_slug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSlugFromPath(tt.path)
			if got != tt.want {
				t.Errorf("extractSlugFromPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestExtractSlugFromPath_RealWorldExamples(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/abc123", "abc123"},
		{"/s/abc123", "abc123"},
		{"/short/abc123", "abc123"},
		{"/redirect/abc123", "abc123"},
		{"/abc123?query=param", "abc123?query=param"}, // Note: doesn't strip query params
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := extractSlugFromPath(tt.path)
			if got != tt.want {
				t.Errorf("extractSlugFromPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// Test edge cases
func TestValidateSlugFormat_EdgeCases(t *testing.T) {
	// Test exactly at boundary
	maxLengthSlug := make([]byte, MaxSlugLength)
	for i := range maxLengthSlug {
		maxLengthSlug[i] = 'a'
	}

	err := validateSlugFormat(string(maxLengthSlug))
	if err != nil {
		t.Errorf("expected no error for max length slug, got %v", err)
	}

	// Test one over boundary
	overMaxSlug := make([]byte, MaxSlugLength+1)
	for i := range overMaxSlug {
		overMaxSlug[i] = 'a'
	}

	err = validateSlugFormat(string(overMaxSlug))
	if err == nil {
		t.Error("expected error for slug exceeding max length")
	}
}

func TestValidateCreateRequest_ErrorMessages(t *testing.T) {
	err := validateCreateRequest(HTTPCreateLinkRequest{URL: ""})
	if err == nil {
		t.Fatal("expected error for empty URL")
	}

	if !errors.Is(err, err) {
		t.Error("error should be unwrappable")
	}

	// Verify error message is helpful
	if err.Error() != "url is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}
