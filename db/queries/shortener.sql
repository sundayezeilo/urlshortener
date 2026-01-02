-- name: CreateLink :one
INSERT INTO links (
    id,
    original_url,
    slug
) VALUES (
    $1, $2, $3
)
RETURNING
    id,
    original_url,
    slug,
    access_count,
    created_at,
    updated_at,
    last_accessed_at,
    deleted_at;

-- name: GetLinkBySLug :one
SELECT
    id,
    original_url,
    slug,
    access_count,
    created_at,
    updated_at,
    last_accessed_at,
    deleted_at
FROM links
WHERE slug = $1
  AND deleted_at IS NULL;

-- name: ResolveAndTrack :one
UPDATE links
SET
  access_count     = access_count + 1,
  last_accessed_at = now()
WHERE slug = $1
  AND deleted_at IS NULL
RETURNING
  id,
  original_url,
  slug,
  access_count,
  created_at,
  updated_at,
  last_accessed_at,
  deleted_at;

-- name: ListLinks :many
SELECT
    id,
    original_url,
    slug,
    access_count,
    created_at,
    updated_at,
    last_accessed_at,
    deleted_at
FROM links
WHERE deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: SoftDeleteLink :exec
UPDATE links
SET
    deleted_at = now(),
    updated_at = now()
WHERE slug = $1
  AND deleted_at IS NULL;
