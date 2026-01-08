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
	"github.com/sundayezeilo/urlshortener/internal/idgen"
)

/***************
 * Mocks / Stubs
 ***************/

// mockQueries implements the querier interface for testing.
type mockQueries struct {
	createLinkFunc      func(ctx context.Context, params db.CreateLinkParams) (db.Link, error)
	getLinkBySlugFunc   func(ctx context.Context, slug string) (db.Link, error)
	resolveAndTrackFunc func(ctx context.Context, slug string) (db.Link, error)
	deleteLinkFunc      func(ctx context.Context, slug string) error
}

func (m *mockQueries) CreateLink(ctx context.Context, params db.CreateLinkParams) (db.Link, error) {
	if m.createLinkFunc != nil {
		return m.createLinkFunc(ctx, params)
	}
	return db.Link{}, nil
}

func (m *mockQueries) GetLinkBySLug(ctx context.Context, slug string) (db.Link, error) {
	if m.getLinkBySlugFunc != nil {
		return m.getLinkBySlugFunc(ctx, slug)
	}
	return db.Link{}, nil
}

func (m *mockQueries) ResolveAndTrackLink(ctx context.Context, slug string) (db.Link, error) {
	if m.resolveAndTrackFunc != nil {
		return m.resolveAndTrackFunc(ctx, slug)
	}
	return db.Link{}, nil
}

func (m *mockQueries) DeleteLink(ctx context.Context, slug string) error {
	if m.deleteLinkFunc != nil {
		return m.deleteLinkFunc(ctx, slug)
	}
	return nil
}

// stubIDGen lets tests control generated IDs deterministically.
type stubIDGen struct {
	id    uuid.UUID
	err   error
	calls int
}

func (g *stubIDGen) Generate() (uuid.UUID, error) {
	g.calls++
	return g.id, g.err
}

/***************
 * Helpers
 ***************/

func makeValidTimestamp(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func makeInvalidTimestamp() pgtype.Timestamptz {
	return pgtype.Timestamptz{Valid: false}
}

func makeTestDBLink(now time.Time) db.Link {
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

func makeTestLink(now time.Time) Link {
	return Link{
		ID:             uuid.Nil,
		OriginalURL:    "https://example.com",
		Slug:           "test-slug",
		AccessCount:    0,
		CreatedAt:      now,
		UpdatedAt:      now,
		LastAccessedAt: nil,
	}
}

// makeUUIDv7Deterministic returns a UUID with version bits set to 7.
// (We don't rely on uuid.NewV7 inside tests to avoid version/support surprises.)
func makeUUIDv7Deterministic() uuid.UUID {
	var id uuid.UUID
	// Any bytes are fine as long as we set:
	// - version nibble to 7 at byte[6] high nibble
	// - variant to RFC4122 at byte[8] high bits: 10xxxxxx
	copy(id[:], []byte{
		0x01, 0x23, 0x45, 0x67,
		0x89, 0xab,
		0x70, 0xcd, // 0x7? => version 7
		0x80, 0xef, // 0x8? => RFC4122 variant
		0x10, 0x32, 0x54, 0x76, 0x98, 0xba,
	})
	return id
}

/***************
 * Unit tests: helpers
 ***************/

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
		pgErr := &pgconn.PgError{Code: "42P01", Message: "relation does not exist"}
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

/***************
 * Unit tests: repo methods
 ***************/

func TestRepoCreate(t *testing.T) {
	t.Run("generates ID (UUIDv7) when link.ID is zero and creates successfully", func(t *testing.T) {
		now := time.Now()
		wantID := makeUUIDv7Deterministic()
		gen := &stubIDGen{id: wantID}

		dbLink := makeTestDBLink(now)
		dbLink.ID = wantID // ensure toDomainLink returns same ID we injected

		mock := &mockQueries{
			createLinkFunc: func(_ context.Context, params db.CreateLinkParams) (db.Link, error) {
				if params.ID != wantID {
					t.Errorf("params.ID = %v, want %v", params.ID, wantID)
				}
				if params.Slug != "test-slug" {
					t.Errorf("params.Slug = %q, want %q", params.Slug, "test-slug")
				}
				if params.OriginalUrl != "https://example.com" {
					t.Errorf("params.OriginalUrl = %q, want %q", params.OriginalUrl, "https://example.com")
				}
				return dbLink, nil
			},
		}

		r := NewRepository(mock, &RepositoryConfig{IDGenerator: gen})

		link := makeTestLink(now) // ID is uuid.Nil
		got, err := r.Create(context.Background(), link)
		if err != nil {
			t.Fatalf("Create() unexpected error: %v", err)
		}

		if gen.calls != 1 {
			t.Fatalf("IDGenerator calls=%d want 1", gen.calls)
		}
		if got.ID != wantID {
			t.Errorf("created.ID=%v want %v", got.ID, wantID)
		}
		if got.ID.Version() != 7 {
			t.Errorf("created.ID version=%d want 7", got.ID.Version())
		}
	})

	t.Run("respects pre-set ID in link (does not call generator)", func(t *testing.T) {
		now := time.Now()
		presetID := uuid.New()
		gen := &stubIDGen{id: makeUUIDv7Deterministic()}

		mock := &mockQueries{
			createLinkFunc: func(_ context.Context, params db.CreateLinkParams) (db.Link, error) {
				if params.ID != presetID {
					t.Errorf("CreateLink should use preset ID=%v, got %v", presetID, params.ID)
				}
				return makeTestDBLink(now), nil
			},
		}

		r := NewRepository(mock, &RepositoryConfig{IDGenerator: gen})

		link := makeTestLink(now)
		link.ID = presetID

		_, err := r.Create(context.Background(), link)
		if err != nil {
			t.Fatalf("Create() unexpected error: %v", err)
		}
		if gen.calls != 0 {
			t.Fatalf("generator was called %d times, want 0", gen.calls)
		}
	})

	t.Run("returns Conflict error for duplicate slug", func(t *testing.T) {
		now := time.Now()
		pgErr := &pgconn.PgError{Code: "23505", ConstraintName: "links_slug_unique_active"}

		mock := &mockQueries{
			createLinkFunc: func(_ context.Context, _ db.CreateLinkParams) (db.Link, error) {
				return db.Link{}, pgErr
			},
		}

		r := NewRepository(mock, &RepositoryConfig{IDGenerator: &stubIDGen{id: makeUUIDv7Deterministic()}})
		_, err := r.Create(context.Background(), makeTestLink(now))
		if err == nil {
			t.Fatal("Create() expected error, got nil")
		}
		if errx.KindOf(err) != errx.Conflict {
			t.Errorf("KindOf(err)=%v want %v", errx.KindOf(err), errx.Conflict)
		}
		if errx.OpOf(err) != "shortener.repo.Create" {
			t.Errorf("OpOf(err)=%q want %q", errx.OpOf(err), "shortener.repo.Create")
		}
	})

	t.Run("returns Unavailable error when generator fails", func(t *testing.T) {
		now := time.Now()
		genErr := errors.New("entropy unavailable")
		gen := &stubIDGen{id: uuid.Nil, err: genErr}

		mock := &mockQueries{
			createLinkFunc: func(_ context.Context, _ db.CreateLinkParams) (db.Link, error) {
				t.Fatal("CreateLink should not be called when generator fails")
				return db.Link{}, nil
			},
		}

		r := NewRepository(mock, &RepositoryConfig{IDGenerator: gen})
		_, err := r.Create(context.Background(), makeTestLink(now))
		if err == nil {
			t.Fatal("Create() expected error, got nil")
		}
		if errx.KindOf(err) != errx.Unavailable {
			t.Errorf("KindOf(err)=%v want %v", errx.KindOf(err), errx.Unavailable)
		}
		if errx.OpOf(err) != "shortener.repo.Create" {
			t.Errorf("OpOf(err)=%q want %q", errx.OpOf(err), "shortener.repo.Create")
		}
	})

	t.Run("returns error when toDomainLink fails", func(t *testing.T) {
		now := time.Now()
		invalid := db.Link{
			ID:          uuid.New(),
			OriginalUrl: "https://example.com",
			Slug:        "test",
			CreatedAt:   makeInvalidTimestamp(),
			UpdatedAt:   makeValidTimestamp(now),
		}

		mock := &mockQueries{
			createLinkFunc: func(_ context.Context, _ db.CreateLinkParams) (db.Link, error) {
				return invalid, nil
			},
		}

		r := NewRepository(mock, &RepositoryConfig{IDGenerator: &stubIDGen{id: makeUUIDv7Deterministic()}})
		_, err := r.Create(context.Background(), makeTestLink(now))
		if err == nil {
			t.Fatal("Create() expected error from toDomainLink, got nil")
		}
	})
}

func TestRepoGetBySlug(t *testing.T) {
	t.Run("retrieves link successfully", func(t *testing.T) {
		now := time.Now()
		dbLink := makeTestDBLink(now)

		mock := &mockQueries{
			getLinkBySlugFunc: func(_ context.Context, slug string) (db.Link, error) {
				if slug != "test-slug" {
					t.Errorf("slug=%q want %q", slug, "test-slug")
				}
				return dbLink, nil
			},
		}

		r := NewRepository(mock, &RepositoryConfig{IDGenerator: &stubIDGen{id: makeUUIDv7Deterministic()}})

		got, err := r.GetBySlug(context.Background(), "test-slug")
		if err != nil {
			t.Fatalf("GetBySlug() unexpected error: %v", err)
		}
		if got.Slug != dbLink.Slug {
			t.Errorf("Slug=%q want %q", got.Slug, dbLink.Slug)
		}
	})

	t.Run("returns NotFound for non-existent slug", func(t *testing.T) {
		mock := &mockQueries{
			getLinkBySlugFunc: func(_ context.Context, _ string) (db.Link, error) {
				return db.Link{}, pgx.ErrNoRows
			},
		}

		r := NewRepository(mock, &RepositoryConfig{IDGenerator: &stubIDGen{id: makeUUIDv7Deterministic()}})

		_, err := r.GetBySlug(context.Background(), "missing")
		if err == nil {
			t.Fatal("expected error")
		}
		if errx.KindOf(err) != errx.NotFound {
			t.Errorf("KindOf(err)=%v want %v", errx.KindOf(err), errx.NotFound)
		}
		if errx.OpOf(err) != "shortener.repo.GetBySlug" {
			t.Errorf("OpOf(err)=%q want %q", errx.OpOf(err), "shortener.repo.GetBySlug")
		}
	})
}

func TestRepoResolveAndTrack(t *testing.T) {
	t.Run("resolves and tracks successfully", func(t *testing.T) {
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
			resolveAndTrackFunc: func(_ context.Context, slug string) (db.Link, error) {
				if slug != "test-slug" {
					t.Errorf("slug=%q want %q", slug, "test-slug")
				}
				return dbLink, nil
			},
		}

		r := NewRepository(mock, &RepositoryConfig{IDGenerator: &stubIDGen{id: makeUUIDv7Deterministic()}})

		got, err := r.ResolveAndTrack(context.Background(), "test-slug")
		if err != nil {
			t.Fatalf("ResolveAndTrack() unexpected error: %v", err)
		}
		if got.AccessCount != 1 {
			t.Errorf("AccessCount=%d want %d", got.AccessCount, 1)
		}
		if got.LastAccessedAt == nil {
			t.Error("LastAccessedAt=nil want non-nil")
		}
	})

	t.Run("returns NotFound for non-existent slug", func(t *testing.T) {
		mock := &mockQueries{
			resolveAndTrackFunc: func(_ context.Context, _ string) (db.Link, error) {
				return db.Link{}, pgx.ErrNoRows
			},
		}

		r := NewRepository(mock, &RepositoryConfig{IDGenerator: &stubIDGen{id: makeUUIDv7Deterministic()}})

		_, err := r.ResolveAndTrack(context.Background(), "missing")
		if err == nil {
			t.Fatal("expected error")
		}
		if errx.KindOf(err) != errx.NotFound {
			t.Errorf("KindOf(err)=%v want %v", errx.KindOf(err), errx.NotFound)
		}
		if errx.OpOf(err) != "shortener.repo.ResolveAndTrack" {
			t.Errorf("OpOf(err)=%q want %q", errx.OpOf(err), "shortener.repo.ResolveAndTrack")
		}
	})
}

func TestRepoDelete(t *testing.T) {
	t.Run("deletes successfully", func(t *testing.T) {
		mock := &mockQueries{
			deleteLinkFunc: func(_ context.Context, slug string) error {
				if slug != "test-slug" {
					t.Errorf("slug=%q want %q", slug, "test-slug")
				}
				return nil
			},
		}

		r := NewRepository(mock, &RepositoryConfig{IDGenerator: &stubIDGen{id: makeUUIDv7Deterministic()}})

		if err := r.Delete(context.Background(), "test-slug"); err != nil {
			t.Fatalf("Delete() unexpected error: %v", err)
		}
	})

	t.Run("returns NotFound for missing slug", func(t *testing.T) {
		mock := &mockQueries{
			deleteLinkFunc: func(_ context.Context, _ string) error {
				return pgx.ErrNoRows
			},
		}

		r := NewRepository(mock, &RepositoryConfig{IDGenerator: &stubIDGen{id: makeUUIDv7Deterministic()}})

		err := r.Delete(context.Background(), "missing")
		if err == nil {
			t.Fatal("expected error")
		}
		if errx.KindOf(err) != errx.NotFound {
			t.Errorf("KindOf(err)=%v want %v", errx.KindOf(err), errx.NotFound)
		}
		if errx.OpOf(err) != "shortener.repo.Delete" {
			t.Errorf("OpOf(err)=%q want %q", errx.OpOf(err), "shortener.repo.Delete")
		}
	})
}

/***************
 * Constructor tests (UUIDv7 default)
 ***************/

func TestNewRepository_DefaultsToUUIDv7(t *testing.T) {
	now := time.Now()

	var captured uuid.UUID
	mock := &mockQueries{
		createLinkFunc: func(_ context.Context, params db.CreateLinkParams) (db.Link, error) {
			captured = params.ID

			return db.Link{
				ID:             params.ID,
				OriginalUrl:    params.OriginalUrl,
				Slug:           params.Slug,
				AccessCount:    0,
				CreatedAt:      makeValidTimestamp(now),
				UpdatedAt:      makeValidTimestamp(now),
				LastAccessedAt: makeInvalidTimestamp(),
			}, nil
		},
	}

	repo := NewRepository(mock, nil) // nil config => default generator

	created, err := repo.Create(context.Background(), Link{
		OriginalURL: "https://example.com",
		Slug:        "abc",
	})
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}

	if created.ID == uuid.Nil {
		t.Fatal("expected non-zero ID")
	}

	if captured.Version() != 7 {
		t.Fatalf("default generator UUID version=%d want 7", captured.Version())
	}
}

func TestNewRepository_AllowsCustomGenerator(t *testing.T) {
	now := time.Now()

	wantID := makeUUIDv7Deterministic()
	gen := &stubIDGen{id: wantID}

	var captured uuid.UUID
	mock := &mockQueries{
		createLinkFunc: func(_ context.Context, params db.CreateLinkParams) (db.Link, error) {
			captured = params.ID
			return db.Link{
				ID:             params.ID,
				OriginalUrl:    params.OriginalUrl,
				Slug:           params.Slug,
				AccessCount:    0,
				CreatedAt:      makeValidTimestamp(now),
				UpdatedAt:      makeValidTimestamp(now),
				LastAccessedAt: makeInvalidTimestamp(),
			}, nil
		},
	}

	repo := NewRepository(mock, &RepositoryConfig{IDGenerator: gen})

	created, err := repo.Create(context.Background(), Link{
		OriginalURL: "https://example.com",
		Slug:        "abc",
	})
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}

	if gen.calls != 1 {
		t.Fatalf("generator calls=%d want 1", gen.calls)
	}
	if captured != wantID {
		t.Fatalf("captured=%v want %v", captured, wantID)
	}
	if created.ID != wantID {
		t.Fatalf("created.ID=%v want %v", created.ID, wantID)
	}
}

// Optional: sanity check for idgen.NewV7 itself (not strictly repo-level).
func TestIDGen_NewV7_Sanity(t *testing.T) {
	gen := idgen.NewV7(idgen.WithRetries(0))

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
