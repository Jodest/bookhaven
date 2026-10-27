ALTER TABLE screen_works
    ADD COLUMN poster_source      VARCHAR(16)
        CHECK (poster_source IN ('manual', 'commons', 'tmdb', 'removed')),
    ADD COLUMN poster_url         TEXT,
    ADD COLUMN poster_path        VARCHAR(80),
    ADD COLUMN poster_author      TEXT,
    ADD COLUMN poster_license     VARCHAR(100),
    ADD COLUMN poster_license_url TEXT,
    ADD COLUMN poster_source_url  TEXT,
    ADD COLUMN poster_updated_at  TIMESTAMPTZ,
    ADD CONSTRAINT chk_screen_works_poster CHECK (
        (poster_source IS NULL OR poster_source = 'removed')
            AND poster_url IS NULL AND poster_path IS NULL
     OR poster_source = 'manual' AND poster_path IS NOT NULL AND poster_url IS NULL
     OR poster_source IN ('commons', 'tmdb') AND poster_url IS NOT NULL AND poster_path IS NULL);

CREATE INDEX idx_screen_works_poster_path ON screen_works (poster_path)
    WHERE poster_path IS NOT NULL;
