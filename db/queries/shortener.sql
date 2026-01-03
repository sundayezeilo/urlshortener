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
    last_accessed_at;

-- name: GetLinkBySLug :one
SELECT
    id,
    original_url,
    slug,
    access_count,
    created_at,
    updated_at,
    last_accessed_at
FROM links
WHERE slug = $1;

-- name: ResolveAndTrack :one
UPDATE links
SET
  access_count     = access_count + 1,
  last_accessed_at = now()
WHERE slug = $1
RETURNING
  id,
  original_url,
  slug,
  access_count,
  created_at,
  updated_at,
  last_accessed_at;

-- name: DeleteLink :exec
DELETE FROM links
WHERE slug = $1;
