package idgen

import (
	"testing"
	"time"

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

	t.Run("generates roughly time-ordered UUIDs (sanity check)", func(t *testing.T) {
		gen := NewV7()

		// v7 is designed to be sortable by creation time. We don't want flaky tests,
		// so we add a small sleep to separate timestamps.
		id1, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() err=%v", err)
		}
		time.Sleep(2 * time.Millisecond)

		id2, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() err=%v", err)
		}

		// uuid.UUID compares lexicographically by bytes.
		// With a time gap, id2 should usually be greater than id1.
		if !(string(id2[:]) > string(id1[:])) {
			// Don't hard-fail if you prefer: but with the sleep this should be stable.
			t.Fatalf("expected id2 to sort after id1; id1=%v id2=%v", id1, id2)
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
