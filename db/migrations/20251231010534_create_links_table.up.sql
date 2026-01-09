CREATE TABLE links (
    id               UUID PRIMARY KEY,
    original_url     TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE CHECK (char_length(slug) BETWEEN 7 AND 64),
    access_count     BIGINT NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_accessed_at TIMESTAMPTZ
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
