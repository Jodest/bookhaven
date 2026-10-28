ALTER TABLE screen_works
    ADD COLUMN poster_copy_source_url      TEXT,
    ADD COLUMN poster_copy_path            VARCHAR(80),
    ADD COLUMN poster_copy_status          VARCHAR(16)
        CHECK (poster_copy_status IN ('pending', 'stored', 'failed')),
    ADD COLUMN poster_copy_error           VARCHAR(32),
    ADD COLUMN poster_copy_attempts        INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN poster_copy_next_attempt_at TIMESTAMPTZ,
    ADD COLUMN poster_copy_fetched_at      TIMESTAMPTZ,
    ADD COLUMN poster_copy_checked_at      TIMESTAMPTZ,
    ADD COLUMN poster_tmdb_ref             VARCHAR(24)
        CHECK (poster_tmdb_ref ~ '^(movie|tv)/[0-9]{1,10}$'),
    ADD COLUMN poster_verified_at          TIMESTAMPTZ,
    ADD COLUMN poster_refresh_at           TIMESTAMPTZ;

CREATE INDEX idx_screen_works_poster_copy_due ON screen_works (poster_copy_next_attempt_at)
    WHERE poster_source IN ('commons', 'tmdb');
CREATE INDEX idx_screen_works_poster_copy_path ON screen_works (poster_copy_path)
    WHERE poster_copy_path IS NOT NULL;
CREATE INDEX idx_screen_works_poster_refresh ON screen_works (poster_refresh_at)
    WHERE poster_source = 'tmdb';

UPDATE screen_works
   SET poster_url = 'https://image.tmdb.org/t/p/w500/' || substr(poster_url, length('https://image.tmdb.org/t/p/w342/') + 1)
 WHERE poster_source = 'tmdb' AND poster_url LIKE 'https://image.tmdb.org/t/p/w342/%';
