package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const (
	// MaxRequestBodySize is the maximum allowed request body size (1MB).
	MaxRequestBodySize = 1 << 20
)

// DecodeJSON decodes JSON from the request body with size limits and validation.
// Type parameter T must be a pointer type (e.g., *CreateLinkRequest).
func DecodeJSON[T any](r *http.Request) (T, error) {
	var zeroValue T

	r.Body = http.MaxBytesReader(nil, r.Body, MaxRequestBodySize)
	defer func (){
		_ = r.Body.Close()	// Just to ignore golint warning
	}()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var v T
	if err := decoder.Decode(&v); err != nil {
		var syntaxErr *json.SyntaxError
		var unmarshalErr *json.UnmarshalTypeError
		var maxBytesErr *http.MaxBytesError

		switch {
		case errors.As(err, &syntaxErr):
			return zeroValue, fmt.Errorf("malformed JSON at position %d", syntaxErr.Offset)
		case errors.As(err, &unmarshalErr):
			return zeroValue, fmt.Errorf("invalid value for field %q", unmarshalErr.Field)
		case errors.As(err, &maxBytesErr):
			return zeroValue, fmt.Errorf("request body too large (max %d bytes)", MaxRequestBodySize)
		case errors.Is(err, io.EOF):
			return zeroValue, errors.New("request body is empty")
		default:
			return zeroValue, fmt.Errorf("failed to decode JSON: %w", err)
		}
	}

	// Ensure there's no additional data after the JSON object
	if decoder.More() {
		return zeroValue, errors.New("request body contains multiple JSON objects")
	}

	return v, nil
}
