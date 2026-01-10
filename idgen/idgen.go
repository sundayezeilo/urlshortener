package idgen

import (
	"crypto/rand"
	"fmt"

	"github.com/google/uuid"
)

// Generator generates unique identifiers.
// Implementations should be safe for concurrent use.
type Generator interface {
	Generate() (uuid.UUID, error)
}

// Version selects a UUID variant.
type Version uint8

const (
	V4 Version = 4
	V7 Version = 7
)

/***************
 * UUID v4
 ***************/

type v4Gen struct{}

// NewV4 returns a Generator that produces UUID v4 values.
func NewV4() Generator { return v4Gen{} }

func (v4Gen) Generate() (uuid.UUID, error) {
	return uuid.New(), nil
}

/***************
 * UUID v7
 ***************/

type v7Gen struct {
	maxRetries int
}

type V7Option func(*v7Gen)

// WithRetries sets how many times to retry uuid.NewV7() after the initial attempt.
// Defaults to 1. Set to 0 to disable retries.
func WithRetries(n int) V7Option {
	return func(g *v7Gen) {
		if n >= 0 {
			g.maxRetries = n
		}
	}
}

// NewV7 returns a Generator that produces UUID v7 values.
func NewV7(opts ...V7Option) Generator {
	g := &v7Gen{maxRetries: 1}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

func (g *v7Gen) Generate() (uuid.UUID, error) {
	var last error
	for attempt := 0; attempt <= g.maxRetries; attempt++ {
		id, err := uuid.NewV7()
		if err == nil {
			return id, nil
		}
		last = err

		_ = rand.Reader
	}
	return uuid.Nil, fmt.Errorf("uuid v7 generation failed after %d attempts: %w", g.maxRetries+1, last)
}

// New returns a Generator for the requested UUID version.
func New(v Version, v7opts ...V7Option) Generator {
	switch v {
	case V7:
		return NewV7(v7opts...)
	default:
		return NewV4()
	}
}
