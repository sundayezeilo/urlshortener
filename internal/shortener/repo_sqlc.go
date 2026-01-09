package shortener

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/sundayezeilo/urlshortener/internal/db/sqlc"
	"github.com/sundayezeilo/urlshortener/internal/errx"
	"github.com/sundayezeilo/urlshortener/internal/idgen"
)

// querier is an internal interface that abstracts *db.Queries
type querier interface {
	CreateLink(ctx context.Context, arg db.CreateLinkParams) (db.Link, error)
	GetLinkBySLug(ctx context.Context, slug string) (db.Link, error)
	ResolveAndTrackLink(ctx context.Context, slug string) (db.Link, error)
	DeleteLink(ctx context.Context, slug string) error
}

type repo struct {
	q   querier
	ids idgen.Generator
}

// RepositoryConfig holds configuration for the repository
type RepositoryConfig struct {
	IDGenerator idgen.Generator
}

// NewRepository creates a new Repository implementation
func NewRepository(q querier, config *RepositoryConfig) Repository {
	if config == nil {
		config = &RepositoryConfig{}
	}

	// Default: UUID v7 (good for DB locality). Retry once by default inside idgen.NewV7.
	if config.IDGenerator == nil {
		config.IDGenerator = idgen.NewV7(idgen.WithRetries(1))
	}

	return &repo{
		q:   q,
		ids: config.IDGenerator,
	}
}

func mustTime(ts pgtype.Timestamptz, field string) (time.Time, error) {
	if !ts.Valid {
		return time.Time{}, fmt.Errorf("%s unexpectedly NULL", field)
	}
	return ts.Time, nil
}

func timePtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

func toDomainLink(x db.Link) (Link, error) {
	createdAt, err := mustTime(x.CreatedAt, "created_at")
	if err != nil {
		return Link{}, err
	}
	updatedAt, err := mustTime(x.UpdatedAt, "updated_at")
	if err != nil {
		return Link{}, err
	}

	return Link{
		ID:             x.ID,
		OriginalURL:    x.OriginalUrl,
		Slug:           x.Slug,
		AccessCount:    x.AccessCount,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
		LastAccessedAt: timePtr(x.LastAccessedAt),
	}, nil
}

func mapRepoError(op string, err error) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return errx.E(op, errx.NotFound, err)

	case isSlugUniqueViolation(err):
		return errx.E(op, errx.Conflict, err)

	default:
		return errx.E(op, errx.Unavailable, err)
	}
}

func (r *repo) Create(ctx context.Context, link Link) (Link, error) {
	const op = "shortener.repo.Create"

	// Generate ID if not provided
	if link.ID == uuid.Nil {
		id, err := r.ids.Generate()
		if err != nil {
			return Link{}, errx.E(op, errx.Unavailable, err)
		}
		link.ID = id
	}

	row, err := r.q.CreateLink(ctx, db.CreateLinkParams{
		ID:          link.ID,
		OriginalUrl: link.OriginalURL,
		Slug:        link.Slug,
	})
	if err != nil {
		return Link{}, mapRepoError(op, err)
	}

	return toDomainLink(row)
}

func (r *repo) GetBySlug(ctx context.Context, slug string) (Link, error) {
	const op = "shortener.repo.GetBySlug"

	row, err := r.q.GetLinkBySLug(ctx, slug)
	if err != nil {
		return Link{}, mapRepoError(op, err)
	}
	return toDomainLink(row)
}

func (r *repo) ResolveAndTrack(ctx context.Context, slug string) (Link, error) {
	const op = "shortener.repo.ResolveAndTrack"

	row, err := r.q.ResolveAndTrackLink(ctx, slug)
	if err != nil {
		return Link{}, mapRepoError(op, err)
	}
	return toDomainLink(row)
}

func (r *repo) Delete(ctx context.Context, slug string) error {
	const op = "shortener.repo.Delete"
	if err := r.q.DeleteLink(ctx, slug); err != nil {
		return mapRepoError(op, err)
	}
	return nil
}
