package idgen

import (
	"testing"

	"github.com/google/uuid"
)

func TestV4_Generate(t *testing.T) {
	t.Run("generates valid UUID v4", func(t *testing.T) {
		gen := NewV4()

		id, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() unexpected error: %v", err)
		}
		if id == uuid.Nil {
			t.Fatal("generated UUID is nil")
		}
		if id.Version() != 4 {
			t.Fatalf("UUID version = %d, want 4", id.Version())
		}
	})

	t.Run("generates distinct values (sanity check)", func(t *testing.T) {
		gen := NewV4()

		seen := make(map[uuid.UUID]struct{}, 50)
		for range 50 {
			id, err := gen.Generate()
			if err != nil {
				t.Fatalf("Generate() unexpected error: %v", err)
			}
			if _, ok := seen[id]; ok {
				t.Fatalf("generated duplicate UUID (extremely unlikely): %v", id)
			}
			seen[id] = struct{}{}
		}
	})
}

func TestV7_Generate(t *testing.T) {
	t.Run("generates valid UUID v7", func(t *testing.T) {
		gen := NewV7()

		id, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() unexpected error: %v", err)
		}
		if id == uuid.Nil {
			t.Fatal("generated UUID is nil")
		}
		if id.Version() != 7 {
			t.Fatalf("UUID version = %d, want 7", id.Version())
		}
	})

	t.Run("accepts custom retry settings (behavioral)", func(t *testing.T) {
		// This test mainly ensures the option path compiles and the generator still works.
		gen := NewV7(WithRetries(0))

		id, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() unexpected error: %v", err)
		}
		if id.Version() != 7 {
			t.Fatalf("UUID version = %d, want 7", id.Version())
		}
	})
}

func TestFactory_New(t *testing.T) {
	t.Run("returns v4 by default", func(t *testing.T) {
		gen := New(0) // unknown -> default to v4

		id, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() unexpected error: %v", err)
		}
		if id.Version() != 4 {
			t.Fatalf("UUID version = %d, want 4", id.Version())
		}
	})

	t.Run("returns v7 when requested", func(t *testing.T) {
		gen := New(V7)

		id, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() unexpected error: %v", err)
		}
		if id.Version() != 7 {
			t.Fatalf("UUID version = %d, want 7", id.Version())
		}
	})
}

// TestIDGen_NewV7_Sanity sanity check for idgen.NewV7 itself.
func TestIDGen_NewV7_Sanity(t *testing.T) {
	gen := NewV7(WithRetries(0))

	id, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() unexpected error: %v", err)
	}
	if id == uuid.Nil {
		t.Fatal("expected non-nil UUID")
	}
	if id.Version() != 7 {
		t.Fatalf("UUID version=%d want 7", id.Version())
	}
}
