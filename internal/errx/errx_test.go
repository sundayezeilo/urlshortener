package errx

import (
	"errors"
	"fmt"
	"testing"
)

// TestE tests the E function constructor
func TestE(t *testing.T) {
	t.Run("returns nil when error is nil", func(t *testing.T) {
		got := E("op", NotFound, nil)
		if got != nil {
			t.Errorf("E() with nil error = %v, want nil", got)
		}
	})

	t.Run("constructs Error with all fields", func(t *testing.T) {
		root := errors.New("root cause")
		err := E("repo.GetBySlug", NotFound, root)

		var e *Error
		if !errors.As(err, &e) {
			t.Fatal("expected error to be of type *errx.Error")
		}

		if got, want := e.Op, "repo.GetBySlug"; got != want {
			t.Errorf("Op = %q, want %q", got, want)
		}
		if got, want := e.Kind, NotFound; got != want {
			t.Errorf("Kind = %v, want %v", got, want)
		}
		if !errors.Is(e.Err, root) {
			t.Errorf("Err = %v, want %v", e.Err, root)
		}
	})

	t.Run("preserves all error kinds", func(t *testing.T) {
		kinds := []Kind{Unknown, NotFound, Conflict, Invalid, Unauthorized, Forbidden, Unavailable}
		root := errors.New("test error")

		for _, kind := range kinds {
			t.Run(fmt.Sprintf("kind_%d", kind), func(t *testing.T) {
				err := E("operation", kind, root)
				if got := KindOf(err); got != kind {
					t.Errorf("KindOf() = %v, want %v", got, kind)
				}
			})
		}
	})
}

// TestError_Error tests the Error method
func TestError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "nil inner error returns op",
			err:  &Error{Op: "handler.Resolve", Kind: NotFound, Err: nil},
			want: "handler.Resolve",
		},
		{
			name: "empty op returns inner error message",
			err:  &Error{Op: "", Kind: Unknown, Err: errors.New("root cause")},
			want: "root cause",
		},
		{
			name: "normal case formats op and error",
			err:  &Error{Op: "service.Resolve", Kind: NotFound, Err: errors.New("root cause")},
			want: "service.Resolve: root cause",
		},
		{
			name: "both empty returns empty op",
			err:  &Error{Op: "", Kind: Unknown, Err: nil},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestError_Unwrap tests error unwrapping
func TestError_Unwrap(t *testing.T) {
	t.Run("unwraps to inner error", func(t *testing.T) {
		root := errors.New("root")
		err := E("repo.GetBySlug", NotFound, root)

		if !errors.Is(err, root) {
			t.Error("errors.Is() failed to identify root error through unwrapping")
		}
	})

	t.Run("supports nested wrapping", func(t *testing.T) {
		root := errors.New("database error")
		layer1 := E("repo.Query", Unavailable, root)
		layer2 := E("service.Get", KindOf(layer1), layer1)
		layer3 := E("handler.Handle", KindOf(layer2), layer2)

		if !errors.Is(layer3, root) {
			t.Error("errors.Is() failed with deeply nested errors")
		}
	})

	t.Run("returns nil when Err is nil", func(t *testing.T) {
		err := &Error{Op: "test", Kind: Unknown, Err: nil}
		if unwrapped := err.Unwrap(); unwrapped != nil {
			t.Errorf("Unwrap() = %v, want nil", unwrapped)
		}
	})
}

// TestKindOf tests kind extraction
func TestKindOf(t *testing.T) {
	t.Run("returns Unknown for standard error", func(t *testing.T) {
		err := errors.New("standard error")
		if got := KindOf(err); got != Unknown {
			t.Errorf("KindOf() = %v, want %v", got, Unknown)
		}
	})

	t.Run("returns Unknown for nil error", func(t *testing.T) {
		if got := KindOf(nil); got != Unknown {
			t.Errorf("KindOf(nil) = %v, want %v", got, Unknown)
		}
	})

	t.Run("extracts kind from single errx.Error", func(t *testing.T) {
		err := E("operation", Conflict, errors.New("conflict"))
		if got := KindOf(err); got != Conflict {
			t.Errorf("KindOf() = %v, want %v", got, Conflict)
		}
	})

	t.Run("extracts kind through wrapping chain", func(t *testing.T) {
		root := errors.New("root")
		repo := E("repo.GetBySlug", NotFound, root)
		service := E("service.Resolve", KindOf(repo), repo)
		handler := E("handler.Resolve", KindOf(service), service)

		if got := KindOf(handler); got != NotFound {
			t.Errorf("KindOf() = %v, want %v", got, NotFound)
		}
	})

	t.Run("finds first Kind in chain with mixed errors", func(t *testing.T) {
		root := errors.New("root")
		wrapped := fmt.Errorf("wrapped: %w", root)
		errxErr := E("operation", Forbidden, wrapped)

		if got := KindOf(errxErr); got != Forbidden {
			t.Errorf("KindOf() = %v, want %v", got, Forbidden)
		}
	})
}

// TestOpOf tests operation extraction
func TestOpOf(t *testing.T) {
	t.Run("returns empty for standard error", func(t *testing.T) {
		err := errors.New("standard error")
		if got := OpOf(err); got != "" {
			t.Errorf("OpOf() = %q, want empty string", got)
		}
	})

	t.Run("returns empty for nil error", func(t *testing.T) {
		if got := OpOf(nil); got != "" {
			t.Errorf("OpOf(nil) = %q, want empty string", got)
		}
	})

	t.Run("extracts op from single errx.Error", func(t *testing.T) {
		err := E("repo.Save", Invalid, errors.New("validation failed"))
		if got, want := OpOf(err), "repo.Save"; got != want {
			t.Errorf("OpOf() = %q, want %q", got, want)
		}
	})

	t.Run("extracts outermost op from chain", func(t *testing.T) {
		root := errors.New("root")
		repo := E("repo.GetBySlug", NotFound, root)
		service := E("service.Resolve", KindOf(repo), repo)
		handler := E("handler.Resolve", KindOf(service), service)

		// errors.As finds the first (outermost) match
		if got, want := OpOf(handler), "handler.Resolve"; got != want {
			t.Errorf("OpOf() = %q, want %q", got, want)
		}
	})

	t.Run("handles empty op in Error struct", func(t *testing.T) {
		err := &Error{Op: "", Kind: Unknown, Err: errors.New("test")}
		if got := OpOf(err); got != "" {
			t.Errorf("OpOf() = %q, want empty string", got)
		}
	})
}

// TestErrorsAs tests errors.As compatibility
func TestErrorsAs(t *testing.T) {
	t.Run("finds errx.Error in error chain", func(t *testing.T) {
		root := errors.New("root")
		err := E("repo.GetBySlug", NotFound, root)

		var e *Error
		if !errors.As(err, &e) {
			t.Fatal("errors.As() = false, want true")
		}
		if e == nil {
			t.Fatal("errors.As() set e to nil, want non-nil")
		}
	})

	t.Run("does not match standard errors", func(t *testing.T) {
		err := errors.New("standard error")

		var e *Error
		if errors.As(err, &e) {
			t.Error("errors.As() = true for standard error, want false")
		}
	})

	t.Run("finds errx.Error through fmt.Errorf wrapping", func(t *testing.T) {
		errxErr := E("operation", Unauthorized, errors.New("auth failed"))
		wrapped := fmt.Errorf("context: %w", errxErr)

		var e *Error
		if !errors.As(wrapped, &e) {
			t.Error("errors.As() failed to find errx.Error through fmt.Errorf wrapping")
		}
	})
}

// TestErrorChain tests complex error chain scenarios
func TestErrorChain(t *testing.T) {
	t.Run("preserves all information through chain", func(t *testing.T) {
		root := errors.New("database connection failed")
		repo := E("repo.Connect", Unavailable, root)
		service := E("service.Initialize", KindOf(repo), repo)
		handler := E("handler.Start", KindOf(service), service)

		// Verify kind propagates
		if got := KindOf(handler); got != Unavailable {
			t.Errorf("KindOf() = %v, want %v", got, Unavailable)
		}

		// Verify outermost op
		if got := OpOf(handler); got != "handler.Start" {
			t.Errorf("OpOf() = %q, want %q", got, "handler.Start")
		}

		// Verify error message includes full chain
		errMsg := handler.Error()
		if errMsg == "" {
			t.Error("Error() returned empty string")
		}

		// Verify root error is accessible
		if !errors.Is(handler, root) {
			t.Error("errors.Is() failed to find root error")
		}
	})
}

func TestKind_String(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{Unknown, "Unknown"},
		{NotFound, "NotFound"},
		{Conflict, "Conflict"},
		{Invalid, "Invalid"},
		{Unauthorized, "Unauthorized"},
		{Forbidden, "Forbidden"},
		{Unavailable, "Unavailable"},
		{Internal, "Internal"},
		{Kind(99), "Kind(99)"}, // Unknown kind value
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.kind.String()
			if got != tt.want {
				t.Errorf("Kind.String() = %q, want %q", got, tt.want)
			}
		})
	}
}
