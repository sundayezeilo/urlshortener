package shortener

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/sundayezeilo/urlshortener/internal/db/sqlc"
	"github.com/sundayezeilo/urlshortener/internal/errx"
)

// mockQueries implements the querier interface for testing
type mockQueries struct {
	createLinkFunc          func(ctx context.Context, params db.CreateLinkParams) (db.Link, error)
	getLinkBySlugFunc       func(ctx context.Context, slug string) (db.Link, error)
	resolveAndTrackLinkFunc func(ctx context.Context, slug string) (db.Link, error)
	deleteLinkFunc          func(ctx context.Context, slug string) error
}

func (m *mockQueries) CreateLink(ctx context.Context, params db.CreateLinkParams) (db.Link, error) {
	if m.createLinkFunc == nil {
		return db.Link{}, nil
	}
	return m.createLinkFunc(ctx, params)
}

func (m *mockQueries) GetLinkBySLug(ctx context.Context, slug string) (db.Link, error) {
	if m.getLinkBySlugFunc == nil {
		return db.Link{}, nil
	}
	return m.getLinkBySlugFunc(ctx, slug)
}

func (m *mockQueries) ResolveAndTrackLink(ctx context.Context, slug string) (db.Link, error) {
	if m.resolveAndTrackLinkFunc == nil {
		return db.Link{}, nil
	}
	return m.resolveAndTrackLinkFunc(ctx, slug)
}

func (m *mockQueries) DeleteLink(ctx context.Context, slug string) error {
	if m.deleteLinkFunc == nil {
		return nil
	}
	return m.deleteLinkFunc(ctx, slug)
}

// newTestRepository creates a repository with a mock querier for testing
func newTestRepository(q querier) Repository {
	return &repo{q: q}
}

// Helper functions for creating test data
func makeValidTimestamp(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:  t,
		Valid: true,
	}
}

func makeInvalidTimestamp() pgtype.Timestamptz {
	return pgtype.Timestamptz{Valid: false}
}

func makeTestDBLink() db.Link {
	now := time.Now()
	return db.Link{
		ID:             uuid.New(),
		OriginalUrl:    "https://example.com",
		Slug:           "test-slug",
		AccessCount:    0,
		CreatedAt:      makeValidTimestamp(now),
		UpdatedAt:      makeValidTimestamp(now),
		LastAccessedAt: makeInvalidTimestamp(),
	}
}

func makeTestLink() Link {
	now := time.Now()
	return Link{
		ID:             uuid.New(),
		OriginalURL:    "https://example.com",
		Slug:           "test-slug",
		AccessCount:    0,
		CreatedAt:      now,
		UpdatedAt:      now,
		LastAccessedAt: nil,
	}
}

// TestMustTime tests the mustTime helper function
func TestMustTime(t *testing.T) {
	t.Run("returns time when timestamp is valid", func(t *testing.T) {
		now := time.Now()
		ts := makeValidTimestamp(now)

		got, err := mustTime(ts, "test_field")
		if err != nil {
			t.Fatalf("mustTime() unexpected error: %v", err)
		}

		if !got.Equal(now) {
			t.Errorf("mustTime() = %v, want %v", got, now)
		}
	})

	t.Run("returns error when timestamp is invalid", func(t *testing.T) {
		ts := makeInvalidTimestamp()

		_, err := mustTime(ts, "test_field")
		if err == nil {
			t.Fatal("mustTime() expected error, got nil")
		}

		want := "test_field unexpectedly NULL"
		if err.Error() != want {
			t.Errorf("mustTime() error = %q, want %q", err.Error(), want)
		}
	})
}

// TestTimePtr tests the timePtr helper function
func TestTimePtr(t *testing.T) {
	t.Run("returns pointer when timestamp is valid", func(t *testing.T) {
		now := time.Now()
		ts := makeValidTimestamp(now)

		got := timePtr(ts)
		if got == nil {
			t.Fatal("timePtr() = nil, want non-nil")
		}

		if !got.Equal(now) {
			t.Errorf("timePtr() = %v, want %v", *got, now)
		}
	})

	t.Run("returns nil when timestamp is invalid", func(t *testing.T) {
		ts := makeInvalidTimestamp()

		got := timePtr(ts)
		if got != nil {
			t.Errorf("timePtr() = %v, want nil", got)
		}
	})
}

// TestToDomainLink tests the toDomainLink conversion function
func TestToDomainLink(t *testing.T) {
	t.Run("converts valid db.Link to domain Link", func(t *testing.T) {
		now := time.Now()
		dbLink := db.Link{
			ID:             uuid.New(),
			OriginalUrl:    "https://example.com",
			Slug:           "test",
			AccessCount:    5,
			CreatedAt:      makeValidTimestamp(now),
			UpdatedAt:      makeValidTimestamp(now),
			LastAccessedAt: makeValidTimestamp(now.Add(-1 * time.Hour)),
		}

		got, err := toDomainLink(dbLink)
		if err != nil {
			t.Fatalf("toDomainLink() unexpected error: %v", err)
		}

		if got.ID != dbLink.ID {
			t.Errorf("ID = %v, want %v", got.ID, dbLink.ID)
		}
		if got.OriginalURL != dbLink.OriginalUrl {
			t.Errorf("OriginalURL = %q, want %q", got.OriginalURL, dbLink.OriginalUrl)
		}
		if got.Slug != dbLink.Slug {
			t.Errorf("Slug = %q, want %q", got.Slug, dbLink.Slug)
		}
		if got.AccessCount != dbLink.AccessCount {
			t.Errorf("AccessCount = %d, want %d", got.AccessCount, dbLink.AccessCount)
		}
		if got.LastAccessedAt == nil {
			t.Error("LastAccessedAt = nil, want non-nil")
		}
	})

	t.Run("handles nil LastAccessedAt", func(t *testing.T) {
		now := time.Now()
		dbLink := db.Link{
			ID:             uuid.New(),
			OriginalUrl:    "https://example.com",
			Slug:           "test",
			AccessCount:    0,
			CreatedAt:      makeValidTimestamp(now),
			UpdatedAt:      makeValidTimestamp(now),
			LastAccessedAt: makeInvalidTimestamp(),
		}

		got, err := toDomainLink(dbLink)
		if err != nil {
			t.Fatalf("toDomainLink() unexpected error: %v", err)
		}

		if got.LastAccessedAt != nil {
			t.Errorf("LastAccessedAt = %v, want nil", got.LastAccessedAt)
		}
	})

	t.Run("returns error when CreatedAt is invalid", func(t *testing.T) {
		now := time.Now()
		dbLink := db.Link{
			ID:          uuid.New(),
			OriginalUrl: "https://example.com",
			Slug:        "test",
			CreatedAt:   makeInvalidTimestamp(),
			UpdatedAt:   makeValidTimestamp(now),
		}

		_, err := toDomainLink(dbLink)
		if err == nil {
			t.Fatal("toDomainLink() expected error for invalid CreatedAt, got nil")
		}
	})

	t.Run("returns error when UpdatedAt is invalid", func(t *testing.T) {
		now := time.Now()
		dbLink := db.Link{
			ID:          uuid.New(),
			OriginalUrl: "https://example.com",
			Slug:        "test",
			CreatedAt:   makeValidTimestamp(now),
			UpdatedAt:   makeInvalidTimestamp(),
		}

		_, err := toDomainLink(dbLink)
		if err == nil {
			t.Fatal("toDomainLink() expected error for invalid UpdatedAt, got nil")
		}
	})
}

// TestMapRepoError tests the mapRepoError function
func TestMapRepoError(t *testing.T) {
	t.Run("maps pgx.ErrNoRows to NotFound", func(t *testing.T) {
		err := mapRepoError("test.op", pgx.ErrNoRows)

		if errx.KindOf(err) != errx.NotFound {
			t.Errorf("KindOf(err) = %v, want %v", errx.KindOf(err), errx.NotFound)
		}
		if errx.OpOf(err) != "test.op" {
			t.Errorf("OpOf(err) = %q, want %q", errx.OpOf(err), "test.op")
		}
	})

	t.Run("maps unique constraint violation to Conflict", func(t *testing.T) {
		pgErr := &pgconn.PgError{
			Code:           "23505",
			ConstraintName: "links_slug_unique_active",
		}

		err := mapRepoError("test.op", pgErr)

		if errx.KindOf(err) != errx.Conflict {
			t.Errorf("KindOf(err) = %v, want %v", errx.KindOf(err), errx.Conflict)
		}
		if errx.OpOf(err) != "test.op" {
			t.Errorf("OpOf(err) = %q, want %q", errx.OpOf(err), "test.op")
		}
	})

	t.Run("maps other postgres errors to Unavailable", func(t *testing.T) {
		pgErr := &pgconn.PgError{
			Code:    "42P01", // undefined_table
			Message: "relation does not exist",
		}

		err := mapRepoError("test.op", pgErr)

		if errx.KindOf(err) != errx.Unavailable {
			t.Errorf("KindOf(err) = %v, want %v", errx.KindOf(err), errx.Unavailable)
		}
	})

	t.Run("maps generic errors to Unavailable", func(t *testing.T) {
		genericErr := errors.New("connection refused")

		err := mapRepoError("test.op", genericErr)

		if errx.KindOf(err) != errx.Unavailable {
			t.Errorf("KindOf(err) = %v, want %v", errx.KindOf(err), errx.Unavailable)
		}
	})
}

// TestRepoCreate tests the Create method
func TestRepoCreate(t *testing.T) {
	t.Run("creates link successfully", func(t *testing.T) {
		dbLink := makeTestDBLink()
		mock := &mockQueries{
			createLinkFunc: func(ctx context.Context, params db.CreateLinkParams) (db.Link, error) {
				if params.Slug != "test-slug" {
					t.Errorf("params.Slug = %q, want %q", params.Slug, "test-slug")
				}
				if params.OriginalUrl != "https://example.com" {
					t.Errorf("params.OriginalUrl = %q, want %q", params.OriginalUrl, "https://example.com")
				}
				return dbLink, nil
			},
		}

		r := newTestRepository(mock)
		link := makeTestLink()

		got, err := r.CreateLink(context.Background(), link)
		if err != nil {
			t.Fatalf("Create() unexpected error: %v", err)
		}

		if got.Slug != dbLink.Slug {
			t.Errorf("Create() Slug = %q, want %q", got.Slug, dbLink.Slug)
		}
	})

	t.Run("returns Conflict error for duplicate slug", func(t *testing.T) {
		pgErr := &pgconn.PgError{
			Code:           "23505",
			ConstraintName: "links_slug_unique_active",
		}

		mock := &mockQueries{
			createLinkFunc: func(ctx context.Context, params db.CreateLinkParams) (db.Link, error) {
				return db.Link{}, pgErr
			},
		}

		r := newTestRepository(mock)
		link := makeTestLink()

		_, err := r.CreateLink(context.Background(), link)
		if err == nil {
			t.Fatal("Create() expected error, got nil")
		}

		if errx.KindOf(err) != errx.Conflict {
			t.Errorf("KindOf(err) = %v, want %v", errx.KindOf(err), errx.Conflict)
		}
		if errx.OpOf(err) != "shortener.repo.Create" {
			t.Errorf("OpOf(err) = %q, want %q", errx.OpOf(err), "shortener.repo.Create")
		}
	})

	t.Run("returns Unavailable error for database error", func(t *testing.T) {
		mock := &mockQueries{
			createLinkFunc: func(ctx context.Context, params db.CreateLinkParams) (db.Link, error) {
				return db.Link{}, errors.New("connection timeout")
			},
		}

		r := newTestRepository(mock)
		link := makeTestLink()

		_, err := r.CreateLink(context.Background(), link)
		if err == nil {
			t.Fatal("Create() expected error, got nil")
		}

		if errx.KindOf(err) != errx.Unavailable {
			t.Errorf("KindOf(err) = %v, want %v", errx.KindOf(err), errx.Unavailable)
		}
	})

	t.Run("returns error when toDomainLink fails", func(t *testing.T) {
		invalidDBLink := db.Link{
			ID:          uuid.New(),
			OriginalUrl: "https://example.com",
			Slug:        "test",
			CreatedAt:   makeInvalidTimestamp(), // Invalid timestamp
			UpdatedAt:   makeValidTimestamp(time.Now()),
		}

		mock := &mockQueries{
			createLinkFunc: func(ctx context.Context, params db.CreateLinkParams) (db.Link, error) {
				return invalidDBLink, nil
			},
		}

		r := newTestRepository(mock)
		link := makeTestLink()

		_, err := r.CreateLink(context.Background(), link)
		if err == nil {
			t.Fatal("Create() expected error from toDomainLink, got nil")
		}
	})
}

// TestRepoGetBySlug tests the GetBySlug method
func TestRepoGetBySlug(t *testing.T) {
	t.Run("retrieves link successfully", func(t *testing.T) {
		dbLink := makeTestDBLink()
		mock := &mockQueries{
			getLinkBySlugFunc: func(ctx context.Context, slug string) (db.Link, error) {
				if slug != "test-slug" {
					t.Errorf("slug = %q, want %q", slug, "test-slug")
				}
				return dbLink, nil
			},
		}

		r := newTestRepository(mock)

		got, err := r.GetLinkBySlug(context.Background(), "test-slug")
		if err != nil {
			t.Fatalf("GetBySlug() unexpected error: %v", err)
		}

		if got.Slug != dbLink.Slug {
			t.Errorf("GetBySlug() Slug = %q, want %q", got.Slug, dbLink.Slug)
		}
	})

	t.Run("returns NotFound error for non-existent slug", func(t *testing.T) {
		mock := &mockQueries{
			getLinkBySlugFunc: func(ctx context.Context, slug string) (db.Link, error) {
				return db.Link{}, pgx.ErrNoRows
			},
		}

		r := newTestRepository(mock)

		_, err := r.GetLinkBySlug(context.Background(), "nonexistent")
		if err == nil {
			t.Fatal("GetBySlug() expected error, got nil")
		}

		if errx.KindOf(err) != errx.NotFound {
			t.Errorf("KindOf(err) = %v, want %v", errx.KindOf(err), errx.NotFound)
		}
		if errx.OpOf(err) != "shortener.repo.GetBySlug" {
			t.Errorf("OpOf(err) = %q, want %q", errx.OpOf(err), "shortener.repo.GetBySlug")
		}
	})

	t.Run("returns Unavailable error for database error", func(t *testing.T) {
		mock := &mockQueries{
			getLinkBySlugFunc: func(ctx context.Context, slug string) (db.Link, error) {
				return db.Link{}, errors.New("connection failed")
			},
		}

		r := newTestRepository(mock)

		_, err := r.GetLinkBySlug(context.Background(), "test-slug")
		if err == nil {
			t.Fatal("GetBySlug() expected error, got nil")
		}

		if errx.KindOf(err) != errx.Unavailable {
			t.Errorf("KindOf(err) = %v, want %v", errx.KindOf(err), errx.Unavailable)
		}
	})
}

// TestRepoResolveAndTrack tests the ResolveAndTrack method
func TestRepoResolveAndTrack(t *testing.T) {
	t.Run("resolves and tracks link successfully", func(t *testing.T) {
		now := time.Now()
		dbLink := db.Link{
			ID:             uuid.New(),
			OriginalUrl:    "https://example.com",
			Slug:           "test-slug",
			AccessCount:    1,
			CreatedAt:      makeValidTimestamp(now),
			UpdatedAt:      makeValidTimestamp(now),
			LastAccessedAt: makeValidTimestamp(now),
		}

		mock := &mockQueries{
			resolveAndTrackLinkFunc: func(ctx context.Context, slug string) (db.Link, error) {
				if slug != "test-slug" {
					t.Errorf("slug = %q, want %q", slug, "test-slug")
				}
				return dbLink, nil
			},
		}

		r := newTestRepository(mock)

		got, err := r.ResolveAndTrackLink(context.Background(), "test-slug")
		if err != nil {
			t.Fatalf("ResolveAndTrack() unexpected error: %v", err)
		}

		if got.AccessCount != 1 {
			t.Errorf("AccessCount = %d, want %d", got.AccessCount, 1)
		}
		if got.LastAccessedAt == nil {
			t.Error("LastAccessedAt = nil, want non-nil")
		}
	})

	t.Run("returns NotFound error for non-existent slug", func(t *testing.T) {
		mock := &mockQueries{
			resolveAndTrackLinkFunc: func(ctx context.Context, slug string) (db.Link, error) {
				return db.Link{}, pgx.ErrNoRows
			},
		}

		r := newTestRepository(mock)

		_, err := r.ResolveAndTrackLink(context.Background(), "nonexistent")
		if err == nil {
			t.Fatal("ResolveAndTrack() expected error, got nil")
		}

		if errx.KindOf(err) != errx.NotFound {
			t.Errorf("KindOf(err) = %v, want %v", errx.KindOf(err), errx.NotFound)
		}
		if errx.OpOf(err) != "shortener.repo.ResolveAndTrack" {
			t.Errorf("OpOf(err) = %q, want %q", errx.OpOf(err), "shortener.repo.ResolveAndTrack")
		}
	})

	t.Run("returns Unavailable error for database error", func(t *testing.T) {
		mock := &mockQueries{
			resolveAndTrackLinkFunc: func(ctx context.Context, slug string) (db.Link, error) {
				return db.Link{}, errors.New("deadlock detected")
			},
		}

		r := newTestRepository(mock)

		_, err := r.ResolveAndTrackLink(context.Background(), "test-slug")
		if err == nil {
			t.Fatal("ResolveAndTrack() expected error, got nil")
		}

		if errx.KindOf(err) != errx.Unavailable {
			t.Errorf("KindOf(err) = %v, want %v", errx.KindOf(err), errx.Unavailable)
		}
	})
}

// TestRepoDeleteLink tests the DeleteLink method
func TestRepoDeleteLink(t *testing.T) {
	t.Run("deletes link successfully", func(t *testing.T) {
		mock := &mockQueries{
			deleteLinkFunc: func(ctx context.Context, slug string) error {
				if slug != "test-slug" {
					t.Errorf("slug = %q, want %q", slug, "test-slug")
				}
				return nil
			},
		}

		r := newTestRepository(mock)

		err := r.DeleteLink(context.Background(), "test-slug")
		if err != nil {
			t.Fatalf("DeleteLink() unexpected error: %v", err)
		}
	})

	t.Run("returns NotFound error for non-existent slug", func(t *testing.T) {
		mock := &mockQueries{
			deleteLinkFunc: func(ctx context.Context, slug string) error {
				return pgx.ErrNoRows
			},
		}

		r := newTestRepository(mock)

		err := r.DeleteLink(context.Background(), "nonexistent")
		if err == nil {
			t.Fatal("DeleteLink() expected error, got nil")
		}

		if errx.KindOf(err) != errx.NotFound {
			t.Errorf("KindOf(err) = %v, want %v", errx.KindOf(err), errx.NotFound)
		}
		if errx.OpOf(err) != "shortener.repo.DeleteLink" {
			t.Errorf("OpOf(err) = %q, want %q", errx.OpOf(err), "shortener.repo.DeleteLink")
		}
	})

	t.Run("returns Unavailable error for database error", func(t *testing.T) {
		mock := &mockQueries{
			deleteLinkFunc: func(ctx context.Context, slug string) error {
				return errors.New("connection lost")
			},
		}

		r := newTestRepository(mock)

		err := r.DeleteLink(context.Background(), "test-slug")
		if err == nil {
			t.Fatal("DeleteLink() expected error, got nil")
		}

		if errx.KindOf(err) != errx.Unavailable {
			t.Errorf("KindOf(err) = %v, want %v", errx.KindOf(err), errx.Unavailable)
		}
	})
}
