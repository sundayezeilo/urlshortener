package shortener

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sundayezeilo/urlshortener/internal/errx"
)

/***************
 * Mocks
 ***************/

// mockRepository implements Repository interface for testing.
type mockRepository struct {
	createFunc          func(ctx context.Context, link Link) (Link, error)
	getBySlugFunc       func(ctx context.Context, slug string) (Link, error)
	resolveAndTrackFunc func(ctx context.Context, slug string) (Link, error)
	deleteFunc          func(ctx context.Context, slug string) error
}

func (m *mockRepository) Create(ctx context.Context, link Link) (Link, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, link)
	}
	link.ID = uuid.New()
	link.CreatedAt = time.Now()
	link.UpdatedAt = time.Now()
	return link, nil
}

func (m *mockRepository) GetBySlug(ctx context.Context, slug string) (Link, error) {
	if m.getBySlugFunc != nil {
		return m.getBySlugFunc(ctx, slug)
	}
	return Link{}, errx.E("repo.GetBySlug", errx.NotFound, errors.New("not found"))
}

func (m *mockRepository) ResolveAndTrack(ctx context.Context, slug string) (Link, error) {
	if m.resolveAndTrackFunc != nil {
		return m.resolveAndTrackFunc(ctx, slug)
	}
	return Link{}, errx.E("repo.ResolveAndTrack", errx.NotFound, errors.New("not found"))
}

func (m *mockRepository) Delete(ctx context.Context, slug string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, slug)
	}
	return nil
}

// mockSlugGenerator implements slug generator for testing.
type mockSlugGenerator struct {
	generateFunc func(length int) (string, error)
	slugs        []string
	callCount    int
}

func (m *mockSlugGenerator) Generate(length int) (string, error) {
	m.callCount++

	if m.generateFunc != nil {
		return m.generateFunc(length)
	}
	if m.slugs != nil {
		idx := m.callCount - 1
		if idx >= 0 && idx < len(m.slugs) {
			return m.slugs[idx], nil
		}
	}
	return "abc1234", nil
}

/***************
 * Constructor Tests
 ***************/

func TestNewService(t *testing.T) {
	t.Run("creates service with nil config", func(t *testing.T) {
		repo := &mockRepository{}
		svc := NewService(repo, nil)
		if svc == nil {
			t.Fatal("NewService() returned nil")
		}
	})

	t.Run("creates service with empty config", func(t *testing.T) {
		repo := &mockRepository{}
		svc := NewService(repo, &ServiceConfig{})
		if svc == nil {
			t.Fatal("NewService() returned nil")
		}
	})

	t.Run("creates service with custom slug generator", func(t *testing.T) {
		repo := &mockRepository{}
		generator := &mockSlugGenerator{}
		svc := NewService(repo, &ServiceConfig{
			SlugGenerator: generator,
			SlugLength:    10,
		})
		if svc == nil {
			t.Fatal("NewService() returned nil")
		}
	})

	t.Run("uses default slug length when below minimum", func(t *testing.T) {
		repo := &mockRepository{}
		svc := NewService(repo, &ServiceConfig{SlugLength: 2})
		if svc == nil {
			t.Fatal("NewService() returned nil")
		}
	})

	t.Run("uses default slug length when above maximum", func(t *testing.T) {
		repo := &mockRepository{}
		svc := NewService(repo, &ServiceConfig{SlugLength: 100})
		if svc == nil {
			t.Fatal("NewService() returned nil")
		}
	})

	t.Run("respects SlugMaxRetries when provided", func(t *testing.T) {
		gen := &mockSlugGenerator{slugs: []string{"a1"}}
		createCalls := 0

		svc := NewService(&mockRepository{
			createFunc: func(ctx context.Context, link Link) (Link, error) {
				createCalls++
				return Link{}, errx.E("repo.Create", errx.Conflict, errors.New("duplicate"))
			},
		}, &ServiceConfig{
			SlugGenerator:  gen,
			SlugMaxRetries: 1,
		})

		_, err := svc.Create(context.Background(), CreateLinkRequest{OriginalURL: "https://example.com"})
		if err == nil {
			t.Fatal("Create() expected error, got nil")
		}
		if errx.KindOf(err) != errx.Unavailable {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Unavailable)
		}
		if createCalls != 1 {
			t.Errorf("Create called %d times, want 1", createCalls)
		}
		if gen.callCount != 1 {
			t.Errorf("Generator called %d times, want 1", gen.callCount)
		}
	})
}

/***************
 * Create Tests
 ***************/

func TestServiceCreate(t *testing.T) {
	t.Run("creates link with custom slug successfully", func(t *testing.T) {
		var capturedLink Link
		repo := &mockRepository{
			createFunc: func(ctx context.Context, link Link) (Link, error) {
				capturedLink = link
				link.ID = uuid.New()
				link.CreatedAt = time.Now()
				link.UpdatedAt = time.Now()
				return link, nil
			},
		}

		svc := NewService(repo, &ServiceConfig{
			SlugGenerator: &mockSlugGenerator{},
		})

		result, err := svc.Create(context.Background(), CreateLinkRequest{
			OriginalURL: "https://example.com",
			CustomSlug:  "my-slug",
		})
		if err != nil {
			t.Fatalf("Create() unexpected error: %v", err)
		}

		if capturedLink.OriginalURL != "https://example.com" {
			t.Errorf("OriginalURL = %q, want %q", capturedLink.OriginalURL, "https://example.com")
		}
		if capturedLink.Slug != "my-slug" {
			t.Errorf("Slug = %q, want %q", capturedLink.Slug, "my-slug")
		}
		if result.ID == uuid.Nil {
			t.Error("returned Link.ID is nil")
		}
	})

	t.Run("creates link with generated slug successfully", func(t *testing.T) {
		var capturedLink Link
		repo := &mockRepository{
			createFunc: func(ctx context.Context, link Link) (Link, error) {
				capturedLink = link
				link.ID = uuid.New()
				link.CreatedAt = time.Now()
				link.UpdatedAt = time.Now()
				return link, nil
			},
		}

		svc := NewService(repo, &ServiceConfig{
			SlugGenerator: &mockSlugGenerator{
				generateFunc: func(length int) (string, error) {
					return "xyz9876", nil
				},
			},
			SlugLength: 7,
		})

		result, err := svc.Create(context.Background(), CreateLinkRequest{
			OriginalURL: "https://example.com",
		})
		if err != nil {
			t.Fatalf("Create() unexpected error: %v", err)
		}

		if capturedLink.Slug != "xyz9876" {
			t.Errorf("Slug = %q, want %q", capturedLink.Slug, "xyz9876")
		}
		if result.Slug != "xyz9876" {
			t.Errorf("returned Slug = %q, want %q", result.Slug, "xyz9876")
		}
	})

	t.Run("retries on Conflict from repository Create and succeeds", func(t *testing.T) {
		createCalls := 0
		var capturedSlugs []string

		repo := &mockRepository{
			createFunc: func(ctx context.Context, link Link) (Link, error) {
				createCalls++
				capturedSlugs = append(capturedSlugs, link.Slug)

				// First attempt: collision
				if createCalls == 1 {
					return Link{}, errx.E("repo.Create", errx.Conflict, errors.New("duplicate slug"))
				}

				// Second attempt: success
				link.ID = uuid.New()
				link.CreatedAt = time.Now()
				link.UpdatedAt = time.Now()
				return link, nil
			},
		}

		gen := &mockSlugGenerator{slugs: []string{"first", "second"}}

		svc := NewService(repo, &ServiceConfig{
			SlugGenerator:  gen,
			SlugLength:     6,
			SlugMaxRetries: 3,
		})

		got, err := svc.Create(context.Background(), CreateLinkRequest{
			OriginalURL: "https://example.com",
		})
		if err != nil {
			t.Fatalf("Create() unexpected error: %v", err)
		}

		if got.Slug != "second" {
			t.Errorf("Slug = %q, want %q", got.Slug, "second")
		}
		if createCalls != 2 {
			t.Errorf("Create called %d times, want 2", createCalls)
		}
		if gen.callCount != 2 {
			t.Errorf("Generator called %d times, want 2", gen.callCount)
		}
		if len(capturedSlugs) != 2 || capturedSlugs[0] != "first" || capturedSlugs[1] != "second" {
			t.Errorf("captured slugs = %#v, want [first second]", capturedSlugs)
		}
	})

	t.Run("returns Unavailable after exhausting retries on Conflict", func(t *testing.T) {
		createCalls := 0
		repo := &mockRepository{
			createFunc: func(ctx context.Context, link Link) (Link, error) {
				createCalls++
				return Link{}, errx.E("repo.Create", errx.Conflict, errors.New("duplicate slug"))
			},
		}

		gen := &mockSlugGenerator{slugs: []string{"a1", "b2", "c3"}}

		svc := NewService(repo, &ServiceConfig{
			SlugGenerator:  gen,
			SlugLength:     2,
			SlugMaxRetries: 3,
		})

		_, err := svc.Create(context.Background(), CreateLinkRequest{
			OriginalURL: "https://example.com",
		})
		if err == nil {
			t.Fatal("Create() expected error, got nil")
		}

		if errx.KindOf(err) != errx.Unavailable {
			t.Errorf("KindOf(err) = %v, want %v", errx.KindOf(err), errx.Unavailable)
		}
		if errx.OpOf(err) != "shortener.service.Create" {
			t.Errorf("OpOf(err) = %q, want %q", errx.OpOf(err), "shortener.service.Create")
		}
		if createCalls != 3 {
			t.Errorf("Create called %d times, want 3", createCalls)
		}
		if gen.callCount != 3 {
			t.Errorf("Generator called %d times, want 3", gen.callCount)
		}
	})

	t.Run("validates URL - empty", func(t *testing.T) {
		svc := NewService(&mockRepository{}, nil)

		_, err := svc.Create(context.Background(), CreateLinkRequest{
			OriginalURL: "",
			CustomSlug:  "valid-slug",
		})
		if err == nil {
			t.Fatal("Create() expected error for empty URL, got nil")
		}
		if errx.KindOf(err) != errx.Invalid {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Invalid)
		}
	})

	t.Run("validates URL - no scheme", func(t *testing.T) {
		svc := NewService(&mockRepository{}, nil)

		_, err := svc.Create(context.Background(), CreateLinkRequest{
			OriginalURL: "example.com",
			CustomSlug:  "valid-slug",
		})
		if err == nil {
			t.Fatal("Create() expected error for URL without scheme, got nil")
		}
		if errx.KindOf(err) != errx.Invalid {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Invalid)
		}
	})

	t.Run("validates URL - wrong scheme", func(t *testing.T) {
		svc := NewService(&mockRepository{}, nil)

		_, err := svc.Create(context.Background(), CreateLinkRequest{
			OriginalURL: "ftp://example.com",
			CustomSlug:  "valid-slug",
		})
		if err == nil {
			t.Fatal("Create() expected error for non-HTTP(S) URL, got nil")
		}
		if errx.KindOf(err) != errx.Invalid {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Invalid)
		}
	})

	t.Run("validates URL - no host", func(t *testing.T) {
		svc := NewService(&mockRepository{}, nil)

		_, err := svc.Create(context.Background(), CreateLinkRequest{
			OriginalURL: "https://",
			CustomSlug:  "valid-slug",
		})
		if err == nil {
			t.Fatal("Create() expected error for URL without host, got nil")
		}
		if errx.KindOf(err) != errx.Invalid {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Invalid)
		}
	})

	t.Run("validates URL - too long", func(t *testing.T) {
		svc := NewService(&mockRepository{}, nil)

		longURL := "https://example.com/" + strings.Repeat("a", 2050)
		_, err := svc.Create(context.Background(), CreateLinkRequest{
			OriginalURL: longURL,
			CustomSlug:  "valid-slug",
		})
		if err == nil {
			t.Fatal("Create() expected error for too long URL, got nil")
		}
		if errx.KindOf(err) != errx.Invalid {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Invalid)
		}
	})

	t.Run("validates custom slug - too short", func(t *testing.T) {
		svc := NewService(&mockRepository{}, nil)

		_, err := svc.Create(context.Background(), CreateLinkRequest{
			OriginalURL: "https://example.com",
			CustomSlug:  "ab",
		})
		if err == nil {
			t.Fatal("Create() expected error for slug too short, got nil")
		}
		if errx.KindOf(err) != errx.Invalid {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Invalid)
		}
	})

	t.Run("validates custom slug - too long", func(t *testing.T) {
		svc := NewService(&mockRepository{}, nil)

		_, err := svc.Create(context.Background(), CreateLinkRequest{
			OriginalURL: "https://example.com",
			CustomSlug:  strings.Repeat("a", 65),
		})
		if err == nil {
			t.Fatal("Create() expected error for slug too long, got nil")
		}
		if errx.KindOf(err) != errx.Invalid {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Invalid)
		}
	})

	t.Run("validates custom slug - starts with dash", func(t *testing.T) {
		svc := NewService(&mockRepository{}, nil)

		_, err := svc.Create(context.Background(), CreateLinkRequest{
			OriginalURL: "https://example.com",
			CustomSlug:  "-invalid",
		})
		if err == nil {
			t.Fatal("Create() expected error for slug starting with dash, got nil")
		}
		if errx.KindOf(err) != errx.Invalid {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Invalid)
		}
	})

	t.Run("validates custom slug - ends with underscore", func(t *testing.T) {
		svc := NewService(&mockRepository{}, nil)

		_, err := svc.Create(context.Background(), CreateLinkRequest{
			OriginalURL: "https://example.com",
			CustomSlug:  "invalid_",
		})
		if err == nil {
			t.Fatal("Create() expected error for slug ending with underscore, got nil")
		}
		if errx.KindOf(err) != errx.Invalid {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Invalid)
		}
	})

	t.Run("validates custom slug - invalid characters", func(t *testing.T) {
		svc := NewService(&mockRepository{}, nil)

		invalidSlugs := []string{
			"abc def",  // space
			"abc@def",  // @
			"abc.def",  // .
			"abc/def",  // /
			"abc\\def", // \
		}

		for _, slug := range invalidSlugs {
			_, err := svc.Create(context.Background(), CreateLinkRequest{
				OriginalURL: "https://example.com",
				CustomSlug:  slug,
			})
			if err == nil {
				t.Errorf("Create() expected error for slug %q, got nil", slug)
				continue
			}
			if errx.KindOf(err) != errx.Invalid {
				t.Errorf("error kind = %v for slug %q, want %v", errx.KindOf(err), slug, errx.Invalid)
			}
		}
	})

	t.Run("accepts valid custom slugs", func(t *testing.T) {
		repo := &mockRepository{}
		svc := NewService(repo, nil)

		validSlugs := []string{
			"abc",
			"abc123",
			"abc-def",
			"abc_def",
			"a1b2c3",
			"ABC-xyz_123",
		}

		for _, slug := range validSlugs {
			_, err := svc.Create(context.Background(), CreateLinkRequest{
				OriginalURL: "https://example.com",
				CustomSlug:  slug,
			})
			if err != nil {
				t.Errorf("Create() unexpected error for valid slug %q: %v", slug, err)
			}
		}
	})

	t.Run("propagates Conflict error from repository for custom slug", func(t *testing.T) {
		repo := &mockRepository{
			createFunc: func(ctx context.Context, link Link) (Link, error) {
				return Link{}, errx.E("repo.Create", errx.Conflict, errors.New("duplicate slug"))
			},
		}
		svc := NewService(repo, nil)

		_, err := svc.Create(context.Background(), CreateLinkRequest{
			OriginalURL: "https://example.com",
			CustomSlug:  "existing",
		})
		if err == nil {
			t.Fatal("Create() expected error from repository, got nil")
		}
		if errx.KindOf(err) != errx.Conflict {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Conflict)
		}
	})

	t.Run("propagates Unavailable error from repository", func(t *testing.T) {
		repo := &mockRepository{
			createFunc: func(ctx context.Context, link Link) (Link, error) {
				return Link{}, errx.E("repo.Create", errx.Unavailable, errors.New("db error"))
			},
		}
		svc := NewService(repo, nil)

		_, err := svc.Create(context.Background(), CreateLinkRequest{
			OriginalURL: "https://example.com",
			CustomSlug:  "valid-slug",
		})
		if err == nil {
			t.Fatal("Create() expected error from repository, got nil")
		}
		if errx.KindOf(err) != errx.Unavailable {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Unavailable)
		}
	})

	t.Run("returns Unavailable when slug generator fails", func(t *testing.T) {
		repo := &mockRepository{}
		generator := &mockSlugGenerator{
			generateFunc: func(length int) (string, error) {
				return "", errors.New("entropy exhausted")
			},
		}
		svc := NewService(repo, &ServiceConfig{SlugGenerator: generator})

		_, err := svc.Create(context.Background(), CreateLinkRequest{
			OriginalURL: "https://example.com",
			CustomSlug:  "",
		})
		if err == nil {
			t.Fatal("Create() expected error when generator fails, got nil")
		}
		if errx.KindOf(err) != errx.Unavailable {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Unavailable)
		}
	})

	t.Run("propagates non-Conflict error from repository during generation", func(t *testing.T) {
		repo := &mockRepository{
			createFunc: func(ctx context.Context, link Link) (Link, error) {
				return Link{}, errx.E("repo.Create", errx.Unavailable, errors.New("db down"))
			},
		}
		svc := NewService(repo, &ServiceConfig{
			SlugGenerator: &mockSlugGenerator{
				generateFunc: func(length int) (string, error) { return "abc123", nil },
			},
		})

		_, err := svc.Create(context.Background(), CreateLinkRequest{
			OriginalURL: "https://example.com",
			CustomSlug:  "",
		})
		if err == nil {
			t.Fatal("Create() expected error from repository, got nil")
		}
		if errx.KindOf(err) != errx.Unavailable {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Unavailable)
		}
	})
}

/***************
 * GetBySlug Tests
 ***************/

func TestServiceGetBySlug(t *testing.T) {
	t.Run("retrieves link successfully", func(t *testing.T) {
		expectedLink := Link{
			ID:          uuid.New(),
			OriginalURL: "https://example.com",
			Slug:        "abc123",
			AccessCount: 5,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		repo := &mockRepository{
			getBySlugFunc: func(ctx context.Context, slug string) (Link, error) {
				if slug != "abc123" {
					t.Errorf("slug = %q, want %q", slug, "abc123")
				}
				return expectedLink, nil
			},
		}

		svc := NewService(repo, nil)

		result, err := svc.GetBySlug(context.Background(), "abc123")
		if err != nil {
			t.Fatalf("GetBySlug() unexpected error: %v", err)
		}

		if result.ID != expectedLink.ID {
			t.Errorf("ID = %v, want %v", result.ID, expectedLink.ID)
		}
		if result.Slug != expectedLink.Slug {
			t.Errorf("Slug = %q, want %q", result.Slug, expectedLink.Slug)
		}
		if result.OriginalURL != expectedLink.OriginalURL {
			t.Errorf("OriginalURL = %q, want %q", result.OriginalURL, expectedLink.OriginalURL)
		}
	})

	t.Run("validates slug - empty", func(t *testing.T) {
		svc := NewService(&mockRepository{}, nil)

		_, err := svc.GetBySlug(context.Background(), "")
		if err == nil {
			t.Fatal("GetBySlug() expected error for empty slug, got nil")
		}

		if errx.KindOf(err) != errx.Invalid {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Invalid)
		}
	})

	t.Run("propagates NotFound error from repository", func(t *testing.T) {
		repo := &mockRepository{
			getBySlugFunc: func(ctx context.Context, slug string) (Link, error) {
				return Link{}, errx.E("repo.GetBySlug", errx.NotFound, errors.New("not found"))
			},
		}

		svc := NewService(repo, nil)

		_, err := svc.GetBySlug(context.Background(), "missing")
		if err == nil {
			t.Fatal("GetBySlug() expected error from repository, got nil")
		}

		if errx.KindOf(err) != errx.NotFound {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.NotFound)
		}
	})

	t.Run("propagates Unavailable error from repository", func(t *testing.T) {
		repo := &mockRepository{
			getBySlugFunc: func(ctx context.Context, slug string) (Link, error) {
				return Link{}, errx.E("repo.GetBySlug", errx.Unavailable, errors.New("db error"))
			},
		}

		svc := NewService(repo, nil)

		_, err := svc.GetBySlug(context.Background(), "abc123")
		if err == nil {
			t.Fatal("GetBySlug() expected error from repository, got nil")
		}

		if errx.KindOf(err) != errx.Unavailable {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Unavailable)
		}
	})
}

/***************
 * Resolve Tests
 ***************/

func TestServiceResolve(t *testing.T) {
	t.Run("resolves slug to URL successfully", func(t *testing.T) {
		expectedURL := "https://example.com/path?query=value"
		repo := &mockRepository{
			resolveAndTrackFunc: func(ctx context.Context, slug string) (Link, error) {
				if slug != "abc123" {
					t.Errorf("slug = %q, want %q", slug, "abc123")
				}
				return Link{
					ID:             uuid.New(),
					OriginalURL:    expectedURL,
					Slug:           slug,
					AccessCount:    10,
					CreatedAt:      time.Now(),
					UpdatedAt:      time.Now(),
					LastAccessedAt: makeTimePtr(time.Now()),
				}, nil
			},
		}

		svc := NewService(repo, nil)

		url, err := svc.Resolve(context.Background(), "abc123")
		if err != nil {
			t.Fatalf("Resolve() unexpected error: %v", err)
		}

		if url != expectedURL {
			t.Errorf("URL = %q, want %q", url, expectedURL)
		}
	})

	t.Run("validates slug - empty", func(t *testing.T) {
		svc := NewService(&mockRepository{}, nil)

		_, err := svc.Resolve(context.Background(), "")
		if err == nil {
			t.Fatal("Resolve() expected error for empty slug, got nil")
		}

		if errx.KindOf(err) != errx.Invalid {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Invalid)
		}
	})

	t.Run("propagates NotFound error from repository", func(t *testing.T) {
		repo := &mockRepository{
			resolveAndTrackFunc: func(ctx context.Context, slug string) (Link, error) {
				return Link{}, errx.E("repo.ResolveAndTrack", errx.NotFound, errors.New("not found"))
			},
		}

		svc := NewService(repo, nil)

		_, err := svc.Resolve(context.Background(), "missing")
		if err == nil {
			t.Fatal("Resolve() expected error from repository, got nil")
		}

		if errx.KindOf(err) != errx.NotFound {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.NotFound)
		}
	})

	t.Run("propagates Unavailable error from repository", func(t *testing.T) {
		repo := &mockRepository{
			resolveAndTrackFunc: func(ctx context.Context, slug string) (Link, error) {
				return Link{}, errx.E("repo.ResolveAndTrack", errx.Unavailable, errors.New("db error"))
			},
		}

		svc := NewService(repo, nil)

		_, err := svc.Resolve(context.Background(), "abc123")
		if err == nil {
			t.Fatal("Resolve() expected error from repository, got nil")
		}

		if errx.KindOf(err) != errx.Unavailable {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Unavailable)
		}
	})
}

/***************
 * Delete Tests
 ***************/

func TestServiceDelete(t *testing.T) {
	t.Run("deletes link successfully", func(t *testing.T) {
		deleted := false
		repo := &mockRepository{
			deleteFunc: func(ctx context.Context, slug string) error {
				if slug != "abc123" {
					t.Errorf("slug = %q, want %q", slug, "abc123")
				}
				deleted = true
				return nil
			},
		}

		svc := NewService(repo, nil)

		err := svc.Delete(context.Background(), "abc123")
		if err != nil {
			t.Fatalf("Delete() unexpected error: %v", err)
		}

		if !deleted {
			t.Error("repository Delete was not called")
		}
	})

	t.Run("validates slug - empty", func(t *testing.T) {
		svc := NewService(&mockRepository{}, nil)

		err := svc.Delete(context.Background(), "")
		if err == nil {
			t.Fatal("Delete() expected error for empty slug, got nil")
		}

		if errx.KindOf(err) != errx.Invalid {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Invalid)
		}
	})

	t.Run("propagates NotFound error from repository", func(t *testing.T) {
		repo := &mockRepository{
			deleteFunc: func(ctx context.Context, slug string) error {
				return errx.E("repo.Delete", errx.NotFound, errors.New("not found"))
			},
		}

		svc := NewService(repo, nil)

		err := svc.Delete(context.Background(), "missing")
		if err == nil {
			t.Fatal("Delete() expected error from repository, got nil")
		}

		if errx.KindOf(err) != errx.NotFound {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.NotFound)
		}
	})

	t.Run("propagates Unavailable error from repository", func(t *testing.T) {
		repo := &mockRepository{
			deleteFunc: func(ctx context.Context, slug string) error {
				return errx.E("repo.Delete", errx.Unavailable, errors.New("db error"))
			},
		}

		svc := NewService(repo, nil)

		err := svc.Delete(context.Background(), "abc123")
		if err == nil {
			t.Fatal("Delete() expected error from repository, got nil")
		}

		if errx.KindOf(err) != errx.Unavailable {
			t.Errorf("error kind = %v, want %v", errx.KindOf(err), errx.Unavailable)
		}
	})
}

/***************
 * Helper Tests
 ***************/

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid http", "http://example.com", false},
		{"valid https", "https://example.com", false},
		{"valid with path", "https://example.com/path", false},
		{"valid with query", "https://example.com?q=test", false},
		{"valid with port", "https://example.com:8080", false},
		{"valid with fragment", "https://example.com#section", false},
		{"empty", "", true},
		{"no scheme", "example.com", true},
		{"invalid scheme", "ftp://example.com", true},
		{"no host", "http://", true},
		{"only scheme", "https://", true},
		{"too long", "https://example.com/" + strings.Repeat("a", 2048), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestValidateSlug(t *testing.T) {
	tests := []struct {
		name    string
		slug    string
		wantErr bool
	}{
		{"valid simple", "abc123", false},
		{"valid with dash", "abc-123", false},
		{"valid with underscore", "abc_123", false},
		{"valid mixed", "Abc-123_XYZ", false},
		{"valid min length", "abc", false},
		{"valid max length", strings.Repeat("a", 64), false},
		{"empty", "", true},
		{"too short", "ab", true},
		{"too long", strings.Repeat("a", 65), true},
		{"starts with dash", "-abc", true},
		{"ends with dash", "abc-", true},
		{"starts with underscore", "_abc", true},
		{"ends with underscore", "abc_", true},
		{"contains space", "abc def", true},
		{"contains @", "abc@def", true},
		{"contains dot", "abc.def", true},
		{"contains slash", "abc/def", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSlug(tt.slug)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSlug(%q) error = %v, wantErr %v", tt.slug, err, tt.wantErr)
			}
		})
	}
}

func TestIsValidSlugChar(t *testing.T) {
	validChars := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_"
	for _, char := range validChars {
		if !isValidSlugChar(char) {
			t.Errorf("isValidSlugChar(%c) = false, want true", char)
		}
	}

	invalidChars := " !@#$%^&*()+=[]{}|;:',.<>?/~`"
	for _, char := range invalidChars {
		if isValidSlugChar(char) {
			t.Errorf("isValidSlugChar(%c) = true, want false", char)
		}
	}
}

// Helper function used in tests
func makeTimePtr(t time.Time) *time.Time {
	return &t
}
