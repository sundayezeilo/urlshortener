package shortener

import (
	"context"
	"crypto/rand"
	"errors"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/sundayezeilo/urlshortener/internal/errx"
)

const (
	DefaultSlugLength = 7
	MaxSlugLength     = 64
	MinSlugLength     = 3
	MaxRetries        = 3
)

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// SlugGenerator defines the interface for slug generation
type SlugGenerator interface {
	Generate(length int) (string, error)
}

// Service defines the business logic operations for URL shortening
type Service interface {
	Create(ctx context.Context, originalURL string, customSlug string) (Link, error)
	GetBySlug(ctx context.Context, slug string) (Link, error)
	Resolve(ctx context.Context, slug string) (string, error)
	Delete(ctx context.Context, slug string) error
}

// service implements the Service interface
type service struct {
	repo          Repository
	slugGenerator SlugGenerator
	slugLength    int
}

// ServiceConfig holds configuration for the service
type ServiceConfig struct {
	SlugGenerator SlugGenerator
	SlugLength    int
}

// NewService creates a new service instance
func NewService(repo Repository, config *ServiceConfig) Service {
	if config == nil {
		config = &ServiceConfig{}
	}

	// Use default slug generator if not provided
	if config.SlugGenerator == nil {
		config.SlugGenerator = &Base62Generator{}
	}

	slugLength := config.SlugLength
	if slugLength < MinSlugLength || slugLength > MaxSlugLength {
		slugLength = DefaultSlugLength
	}

	return &service{
		repo:          repo,
		slugGenerator: config.SlugGenerator,
		slugLength:    slugLength,
	}
}

// Create creates a new short link with optional custom slug
func (s *service) Create(ctx context.Context, originalURL string, customSlug string) (Link, error) {
	const op = "shortener.service.Create"
	var err error

	if err = validateURL(originalURL); err != nil {
		return Link{}, errx.E(op, errx.Invalid, err)
	}

	slug := customSlug
	if slug != "" {
		if err := validateCustomSlug(slug); err != nil {
			return Link{}, errx.E(op, errx.Invalid, err)
		}
	} else {
		slug, err = s.generateUniqueSlug(ctx, op)
		if err != nil {
			return Link{}, err
		}
	}

	id, err := uuid.NewV7()
	if err != nil {
		return Link{}, errx.E(op, errx.Unavailable, err)
	}

	link := Link{
		ID:          id,
		OriginalURL: originalURL,
		Slug:        slug,
	}

	created, err := s.repo.Create(ctx, link)
	if err != nil {
		return Link{}, errx.E(op, errx.KindOf(err), err)
	}

	return created, nil
}

// GetBySlug retrieves a link by its slug
func (s *service) GetBySlug(ctx context.Context, slug string) (Link, error) {
	const op = "shortener.service.GetBySlug"

	if slug == "" {
		return Link{}, errx.E(op, errx.Invalid, errors.New("slug cannot be empty"))
	}

	if len(slug) < MinSlugLength || len(slug) > MaxSlugLength {
		return Link{}, errx.E(op, errx.Invalid, errors.New("slug cannot be empty"))
	}

	link, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return Link{}, errx.E(op, errx.KindOf(err), err)
	}

	return link, nil
}

// Resolve resolves a slug to its original URL and tracks the access
func (s *service) Resolve(ctx context.Context, slug string) (string, error) {
	const op = "shortener.service.Resolve"

	if slug == "" {
		return "", errx.E(op, errx.Invalid, errors.New("slug cannot be empty"))
	}

	link, err := s.repo.ResolveAndTrack(ctx, slug)
	if err != nil {
		return "", errx.E(op, errx.KindOf(err), err)
	}

	return link.OriginalURL, nil
}

// Delete removes a link by its slug
func (s *service) Delete(ctx context.Context, slug string) error {
	const op = "shortener.service.Delete"

	if slug == "" {
		return errx.E(op, errx.Invalid, errors.New("slug cannot be empty"))
	}

	err := s.repo.Delete(ctx, slug)
	if err != nil {
		return errx.E(op, errx.KindOf(err), err)
	}

	return nil
}

// generateUniqueSlug generates a unique slug with retry logic
func (s *service) generateUniqueSlug(ctx context.Context, op string) (string, error) {
	for attempt := 0; attempt < MaxRetries; attempt++ {
		slug, err := s.slugGenerator.Generate(s.slugLength)
		if err != nil {
			return "", errx.E(op, errx.Unavailable, err)
		}

		_, err = s.repo.GetBySlug(ctx, slug)
		if err != nil {
			if errx.KindOf(err) == errx.NotFound {
				return slug, nil
			}
			return "", errx.E(op, errx.KindOf(err), err)
		}
	}

	return "", errx.E(op, errx.Unavailable, errors.New("failed to generate unique slug after maximum retries"))
}

// validateURL validates that the URL is properly formatted
func validateURL(rawURL string) error {
	if rawURL == "" {
		return errors.New("url cannot be empty")
	}

	if len(rawURL) > 2048 {
		return errors.New("url too long (max 2048 characters)")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return errors.New("invalid url format")
	}

	if parsedURL.Scheme == "" {
		return errors.New("url must include scheme (http or https)")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.New("url scheme must be http or https")
	}

	if parsedURL.Host == "" {
		return errors.New("url must include host")
	}

	return nil
}

// validateCustomSlug validates that the slug meets requirements
func validateCustomSlug(slug string) error {
	if slug == "" {
		return errors.New("slug cannot be empty")
	}

	if len(slug) < MinSlugLength {
		return errors.New("slug too short (minimum 3 characters)")
	}

	if len(slug) > MaxSlugLength {
		return errors.New("slug too long (maximum 50 characters)")
	}

	for _, char := range slug {
		if !isValidSlugChar(char) {
			return errors.New("slug contains invalid characters (only alphanumeric, dash, and underscore allowed)")
		}
	}

	// Slug should not start or end with dash/underscore
	if strings.HasPrefix(slug, "-") || strings.HasPrefix(slug, "_") ||
		strings.HasSuffix(slug, "-") || strings.HasSuffix(slug, "_") {
		return errors.New("slug cannot start or end with dash or underscore")
	}

	return nil
}

// isValidSlugChar checks if a character is valid for a slug
func isValidSlugChar(c rune) bool {
	return (c >= '0' && c <= '9') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		c == '-' || c == '_'
}

// Base62Generator implements SlugGenerator using base62 encoding
type Base62Generator struct{}

// Generate generates a random base62 string of the specified length
func (g *Base62Generator) Generate(length int) (string, error) {
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
