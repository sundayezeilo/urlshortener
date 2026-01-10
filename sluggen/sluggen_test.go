package sluggen

import (
	"strings"
	"sync"
	"testing"
)

func TestNewBase62(t *testing.T) {
	gen := NewBase62()
	if gen == nil {
		t.Fatal("NewBase62() returned nil")
	}
}

func TestBase62Generator_Generate(t *testing.T) {
	t.Run("generates slug of correct length", func(t *testing.T) {
		gen := NewBase62()

		lengths := []int{1, 5, 7, 10, 15, 20, 32, 64}
		for _, length := range lengths {
			slug, err := gen.Generate(length)
			if err != nil {
				t.Fatalf("Generate(%d) unexpected error: %v", length, err)
			}

			if len(slug) != length {
				t.Errorf("Generate(%d) returned length %d, want %d", length, len(slug), length)
			}
		}
	})

	t.Run("generates unique slugs", func(t *testing.T) {
		gen := NewBase62()
		seen := make(map[string]bool)

		// Generate 1000 slugs and ensure they're all unique
		for i := 0; i < 1000; i++ {
			slug, err := gen.Generate(10)
			if err != nil {
				t.Fatalf("Generate() unexpected error: %v", err)
			}

			if seen[slug] {
				t.Errorf("Generate() produced duplicate slug: %q", slug)
			}
			seen[slug] = true
		}

		if len(seen) != 1000 {
			t.Errorf("expected 1000 unique slugs, got %d", len(seen))
		}
	})

	t.Run("generates only valid base62 characters", func(t *testing.T) {
		gen := NewBase62()

		// Test with various lengths
		for _, length := range []int{10, 50, 100} {
			slug, err := gen.Generate(length)
			if err != nil {
				t.Fatalf("Generate(%d) unexpected error: %v", length, err)
			}

			for i, char := range slug {
				if !strings.ContainsRune(base62Chars, char) {
					t.Errorf("Generate(%d) produced invalid character %c at position %d", length, char, i)
				}
			}
		}
	})

	t.Run("returns error for zero length", func(t *testing.T) {
		gen := NewBase62()

		_, err := gen.Generate(0)
		if err == nil {
			t.Error("Generate(0) expected error, got nil")
		}

		expectedMsg := "length must be positive"
		if err.Error() != expectedMsg {
			t.Errorf("error message = %q, want %q", err.Error(), expectedMsg)
		}
	})

	t.Run("returns error for negative length", func(t *testing.T) {
		gen := NewBase62()

		_, err := gen.Generate(-1)
		if err == nil {
			t.Error("Generate(-1) expected error, got nil")
		}

		expectedMsg := "length must be positive"
		if err.Error() != expectedMsg {
			t.Errorf("error message = %q, want %q", err.Error(), expectedMsg)
		}
	})

	t.Run("concurrent generation is safe", func(t *testing.T) {
		gen := NewBase62()
		const goroutines = 50
		const iterations = 100

		var wg sync.WaitGroup
		results := make(chan string, goroutines*iterations)
		errChan := make(chan error, goroutines*iterations)

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					slug, err := gen.Generate(8)
					if err != nil {
						errChan <- err
						return
					}
					results <- slug
				}
			}()
		}

		wg.Wait()
		close(results)
		close(errChan)

		// Check for errors
		for err := range errChan {
			t.Errorf("concurrent Generate() error: %v", err)
		}

		// Check for uniqueness
		seen := make(map[string]bool)
		count := 0
		for slug := range results {
			count++
			if seen[slug] {
				t.Errorf("concurrent generation produced duplicate: %q", slug)
			}
			seen[slug] = true
		}

		expectedCount := goroutines * iterations
		if count != expectedCount {
			t.Errorf("expected %d slugs, got %d", expectedCount, count)
		}
	})

	t.Run("generates varied output", func(t *testing.T) {
		gen := NewBase62()

		// Ensure the generator produces varied output (not all the same)
		slugs := make(map[string]int)
		for range 100 {
			slug, err := gen.Generate(10)
			if err != nil {
				t.Fatalf("Generate() unexpected error: %v", err)
			}
			slugs[slug]++
		}

		// All slugs should be unique
		if len(slugs) != 100 {
			t.Errorf("expected 100 unique slugs, got %d", len(slugs))
		}
	})

	t.Run("handles very long slugs", func(t *testing.T) {
		gen := NewBase62()

		slug, err := gen.Generate(1000)
		if err != nil {
			t.Fatalf("Generate(1000) unexpected error: %v", err)
		}

		if len(slug) != 1000 {
			t.Errorf("slug length = %d, want 1000", len(slug))
		}

		// Verify all characters are valid
		for i, char := range slug {
			if !strings.ContainsRune(base62Chars, char) {
				t.Errorf("invalid character %c at position %d", char, i)
				break
			}
		}
	})
}

func TestBase62Chars(t *testing.T) {
	// Verify the base62Chars constant has the expected length
	if len(base62Chars) != 62 {
		t.Errorf("base62Chars length = %d, want 62", len(base62Chars))
	}

	// Verify all characters are unique
	seen := make(map[rune]bool)
	for _, char := range base62Chars {
		if seen[char] {
			t.Errorf("base62Chars contains duplicate character: %c", char)
		}
		seen[char] = true
	}

	// Verify it contains expected character ranges
	expectedChars := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	if base62Chars != expectedChars {
		t.Errorf("base62Chars = %q, want %q", base62Chars, expectedChars)
	}
}

// Benchmark tests
func BenchmarkBase62Generator_Generate(b *testing.B) {
	gen := NewBase62()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := gen.Generate(7)
		if err != nil {
			b.Fatalf("Generate() error: %v", err)
		}
	}
}

func BenchmarkBase62Generator_Generate_Parallel(b *testing.B) {
	gen := NewBase62()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := gen.Generate(7)
			if err != nil {
				b.Fatalf("Generate() error: %v", err)
			}
		}
	})
}
