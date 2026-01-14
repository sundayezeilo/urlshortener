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
	DefaultSlugLength     = 7
	MaxSlugLength         = 64
	MinSlugLength         = 3
	MaxURLLength          = 2048
	DefaultSlugMaxRetries = 3
)

// CreateLinkRequest represents the parameters for creating a new link.
type CreateLinkRequest struct {
	OriginalURL string
	CustomSlug  string // Optional: if empty, a slug will be generated
}

// Service defines the business logic operations for URL shortening.
type Service interface {
	Create(ctx context.Context, req CreateLinkRequest) (Link, error)
	GetBySlug(ctx context.Context, slug string) (Link, error)
	Resolve(ctx context.Context, slug string) (string, error)
	Delete(ctx context.Context, slug string) error
}

// service implements the Service interface.
type service struct {
	repo           Repository
	slugGenerator  sluggen.Generator
	slugLength     int
	slugMaxRetries int
}

// ServiceConfig holds configuration for the service.
type ServiceConfig struct {
	SlugGenerator  sluggen.Generator
	SlugLength     int
	SlugMaxRetries int // attempts when generating a unique slug (default: 3)
}

// NewService creates a new service instance.
func NewService(repo Repository, config *ServiceConfig) Service {
	if config == nil {
		config = &ServiceConfig{}
	}

	slugGen := config.SlugGenerator
	if slugGen == nil {
		slugGen = sluggen.NewBase62()
	}

	slugLength := config.SlugLength
	if slugLength < MinSlugLength || slugLength > MaxSlugLength {
		slugLength = DefaultSlugLength
	}

	retries := config.SlugMaxRetries
	if retries <= 0 {
		retries = DefaultSlugMaxRetries
	}

	return &service{
		repo:           repo,
		slugGenerator:  slugGen,
		slugLength:     slugLength,
		slugMaxRetries: retries,
	}
}

// Create creates a new short link with optional custom slug.
func (s *service) Create(ctx context.Context, req CreateLinkRequest) (Link, error) {
	const op = "shortener.service.Create"

	if err := validateURL(req.OriginalURL); err != nil {
		return Link{}, errx.E(op, errx.Invalid, err)
	}

	// Custom slug path: validate and create once
	if req.CustomSlug != "" {
		if err := validateSlug(req.CustomSlug); err != nil {
			return Link{}, errx.E(op, errx.Invalid, err)
		}

		created, err := s.repo.Create(ctx, Link{
			OriginalURL: req.OriginalURL,
			Slug:        req.CustomSlug,
		})
		if err != nil {
			return Link{}, errx.E(op, errx.KindOf(err), err)
		}
		return created, nil
	}

	// Generated slug path: retry on conflicts
	for range s.slugMaxRetries {
		slug, err := s.slugGenerator.Generate(s.slugLength)
		if err != nil {
			return Link{}, errx.E(op, errx.Unavailable, err)
		}

		created, err := s.repo.Create(ctx, Link{
			OriginalURL: req.OriginalURL,
			Slug:        slug,
		})
		if err == nil {
			return created, nil
		}

		// Retry on conflict, fail on other errors
		if errx.KindOf(err) != errx.Conflict {
			return Link{}, errx.E(op, errx.KindOf(err), err)
		}
	}

	return Link{}, errx.E(op, errx.Unavailable,
		errors.New("could not generate unique slug after retries"))
}

func (s *service) GetBySlug(ctx context.Context, slug string) (Link, error) {
	const op = "shortener.service.GetBySlug"

	if slug == "" {
		return Link{}, errx.E(op, errx.Invalid, errors.New("slug cannot be empty"))
	}

	link, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return Link{}, errx.E(op, errx.KindOf(err), err)
	}
	return link, nil
}

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

func (s *service) Delete(ctx context.Context, slug string) error {
	const op = "shortener.service.Delete"

	if slug == "" {
		return errx.E(op, errx.Invalid, errors.New("slug cannot be empty"))
	}

	if err := s.repo.Delete(ctx, slug); err != nil {
		return errx.E(op, errx.KindOf(err), err)
	}
	return nil
}

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

	if strings.HasPrefix(slug, "-") || strings.HasPrefix(slug, "_") ||
		strings.HasSuffix(slug, "-") || strings.HasSuffix(slug, "_") {
		return errors.New("slug cannot start or end with dash or underscore")
	}

	for _, char := range slug {
		if !isValidSlugChar(char) {
			return errors.New("slug contains invalid characters (only alphanumeric, dash, and underscore allowed)")
		}
	}
	return nil
}

func isValidSlugChar(c rune) bool {
	switch {
	case c >= 'a' && c <= 'z':
		return true
	case c >= 'A' && c <= 'Z':
		return true
	case c >= '0' && c <= '9':
		return true
	case c == '-' || c == '_':
		return true
	default:
		return false
	}
}
