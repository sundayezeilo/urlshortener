// Package sluggen provides slug generation functionality.
// Generators should be safe for concurrent use.
package sluggen

import (
	"crypto/rand"
	"errors"
)

const (
	base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// Generator generates URL slugs.
// Implementations should be safe for concurrent use.
type Generator interface {
	Generate(length int) (string, error)
}

// base62Generator implements Generator using base62 encoding.
// It is safe for concurrent use.
type base62Generator struct{}

// NewBase62 returns a new base62 slug generator.
func NewBase62() Generator {
	return &base62Generator{}
}

// Generate generates a random base62 string of the specified length.
func (g *base62Generator) Generate(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("length must be positive")
	}

	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	for i := range b {
		b[i] = base62Chars[int(b[i])%len(base62Chars)]
	}

	return string(b), nil
}
