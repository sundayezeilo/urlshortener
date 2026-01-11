package shortener

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/sundayezeilo/urlshortener/internal/errx"
	"github.com/sundayezeilo/urlshortener/sluggen"
)

const (
	DefaultSlugLength = 7
	MaxSlugLength     = 64
	MinSlugLength     = 3
	MaxURLLength      = 2048
	MaxRetries        = 3
)

// Service defines the business logic operations for URL shortening.
type Service interface {
	Create(ctx context.Context, originalURL string, customSlug string) (Link, error)
	GetBySlug(ctx context.Context, slug string) (Link, error)
	Resolve(ctx context.Context, slug string) (string, error)
	Delete(ctx context.Context, slug string) error
}

// service implements the Service interface.
type service struct {
	repo          Repository
	slugGenerator sluggen.Generator
	slugLength    int
}

// ServiceConfig holds configuration for the service.
type ServiceConfig struct {
	SlugGenerator sluggen.Generator
	SlugLength    int
}

// NewService creates a new service instance.
func NewService(repo Repository, config *ServiceConfig) Service {
	if config == nil {
		config = &ServiceConfig{}
	}

	// Use default slug generator if not provided
	slugGen := config.SlugGenerator
	if slugGen == nil {
		slugGen = sluggen.NewBase62()
	}

	// Validate and set slug length
	slugLength := config.SlugLength
	if slugLength < MinSlugLength || slugLength > MaxSlugLength {
		slugLength = DefaultSlugLength
	}

	return &service{
		repo:          repo,
		slugGenerator: slugGen,
		slugLength:    slugLength,
	}
}

// Create creates a new short link with optional custom slug.
func (s *service) Create(ctx context.Context, originalURL string, customSlug string) (Link, error) {
	const op = "shortener.service.Create"

	// Validate URL
	if err := validateURL(originalURL); err != nil {
		return Link{}, errx.E(op, errx.Invalid, err)
	}

	// Determine slug
	slug := customSlug
	if slug != "" {
		// Validate custom slug
		if err := validateSlug(slug); err != nil {
			return Link{}, errx.E(op, errx.Invalid, err)
		}
	} else {
		// Generate unique slug
		var err error
		slug, err = s.generateUniqueSlug(ctx)
		if err != nil {
			return Link{}, errx.E(op, errx.KindOf(err), err)
		}
	}

	// Create link (ID generation happens in repository)
	link := Link{
		OriginalURL: originalURL,
		Slug:        slug,
	}

	created, err := s.repo.Create(ctx, link)
	if err != nil {
		return Link{}, errx.E(op, errx.KindOf(err), err)
	}

	return created, nil
}

// GetBySlug retrieves a link by its slug.
func (s *service) GetBySlug(ctx context.Context, slug string) (Link, error) {
	const op = "shortener.service.GetBySlug"

	if err := validateSlugNotEmpty(slug); err != nil {
		return Link{}, errx.E(op, errx.Invalid, err)
	}

	link, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return Link{}, errx.E(op, errx.KindOf(err), err)
	}

	return link, nil
}

// Resolve resolves a slug to its original URL and tracks the access.
func (s *service) Resolve(ctx context.Context, slug string) (string, error) {
	const op = "shortener.service.Resolve"

	if err := validateSlugNotEmpty(slug); err != nil {
		return "", errx.E(op, errx.Invalid, err)
	}

	link, err := s.repo.ResolveAndTrack(ctx, slug)
	if err != nil {
		return "", errx.E(op, errx.KindOf(err), err)
	}

	return link.OriginalURL, nil
}

// Delete removes a link by its slug.
func (s *service) Delete(ctx context.Context, slug string) error {
	const op = "shortener.service.Delete"

	if err := validateSlugNotEmpty(slug); err != nil {
		return errx.E(op, errx.Invalid, err)
	}

	if err := s.repo.Delete(ctx, slug); err != nil {
		return errx.E(op, errx.KindOf(err), err)
	}

	return nil
}

// generateUniqueSlug generates a unique slug with retry logic.
func (s *service) generateUniqueSlug(ctx context.Context) (string, error) {
	for attempt := 0; attempt < MaxRetries; attempt++ {
		slug, err := s.slugGenerator.Generate(s.slugLength)
		if err != nil {
			return "", errx.E("sluggen.Generate", errx.Unavailable, err)
		}

		// Check if slug is available
		_, err = s.repo.GetBySlug(ctx, slug)
		if err != nil {
			if errx.KindOf(err) == errx.NotFound {
				// Slug is available
				return slug, nil
			}
			// Other error occurred (e.g., database unavailable)
			return "", err
		}
		// Slug exists, retry with a new one
	}

	return "", errx.E("sluggen.Generate", errx.Unavailable,
		errors.New("failed to generate unique slug after maximum retries"))
}

// validateURL validates that the URL is properly formatted.
func validateURL(rawURL string) error {
	if rawURL == "" {
		return errors.New("url cannot be empty")
	}

	if len(rawURL) > MaxURLLength {
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

// validateSlug validates that the slug meets all requirements.
func validateSlug(slug string) error {
	if slug == "" {
		return errors.New("slug cannot be empty")
	}

	if len(slug) < MinSlugLength {
		return errors.New("slug too short (minimum 3 characters)")
	}

	if len(slug) > MaxSlugLength {
		return errors.New("slug too long (maximum 64 characters)")
	}

	// Slug should not start or end with dash/underscore
	if strings.HasPrefix(slug, "-") || strings.HasPrefix(slug, "_") ||
		strings.HasSuffix(slug, "-") || strings.HasSuffix(slug, "_") {
		return errors.New("slug cannot start or end with dash or underscore")
	}

	// Check all characters are valid
	for _, char := range slug {
		if !isValidSlugChar(char) {
			return errors.New("slug contains invalid characters (only alphanumeric, dash, and underscore allowed)")
		}
	}

	return nil
}

// validateSlugNotEmpty is a lighter validation for retrieval operations
// where we only need to ensure the slug is not empty.
func validateSlugNotEmpty(slug string) error {
	if slug == "" {
		return errors.New("slug cannot be empty")
	}
	return nil
}

// isValidSlugChar checks if a character is valid for a slug.
func isValidSlugChar(c rune) bool {
	return (c >= '0' && c <= '9') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		c == '-' || c == '_'
}
