package shortener

import "context"

// Repository defines the persistence operations for Link entities.
// It abstracts the underlying data store and is responsible for
// creating, retrieving, updating, and deleting links, as well as
// tracking access-related metadata.
type Repository interface {
	Create(ctx context.Context, link Link) (Link, error)
	GetBySlug(ctx context.Context, slug string) (Link, error)
	ResolveAndTrack(ctx context.Context, slug string) (Link, error)
	Delete(ctx context.Context, slug string) error
}
