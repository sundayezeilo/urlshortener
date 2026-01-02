CREATE TABLE links (
    id               UUID PRIMARY KEY,
    original_url     TEXT NOT NULL,
    slug TEXT NOT NULL CHECK (char_length(slug) <= 32),
    access_count     BIGINT NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_accessed_at TIMESTAMPTZ,
    deleted_at       TIMESTAMPTZ
);

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger AS $$
BEGIN
    IF (NEW IS DISTINCT FROM OLD) THEN
        NEW.updated_at = now();
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER links_set_updated_at
BEFORE UPDATE ON links
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- slug must be unique only among "active" (non-deleted) rows
CREATE UNIQUE INDEX links_slug_unique_active
ON links (slug)
WHERE deleted_at IS NULL;
