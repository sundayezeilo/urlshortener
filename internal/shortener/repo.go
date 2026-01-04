package shortener

import "context"

type Repository interface {
	CreateLink(ctx context.Context, link Link) (Link, error)
	GetLinkBySlug(ctx context.Context, slug string) (Link, error)
	ResolveAndTrackLink(ctx context.Context, slug string) (Link, error)
	DeleteLink(ctx context.Context, slug string) error
}
