UPDATE screen_works
   SET poster_url = 'https://image.tmdb.org/t/p/w342/' || substr(poster_url, length('https://image.tmdb.org/t/p/w500/') + 1)
 WHERE poster_source = 'tmdb' AND poster_url LIKE 'https://image.tmdb.org/t/p/w500/%';

DROP INDEX IF EXISTS idx_screen_works_poster_refresh;
DROP INDEX IF EXISTS idx_screen_works_poster_copy_path;
DROP INDEX IF EXISTS idx_screen_works_poster_copy_due;

ALTER TABLE screen_works
    DROP COLUMN IF EXISTS poster_refresh_at,
    DROP COLUMN IF EXISTS poster_verified_at,
    DROP COLUMN IF EXISTS poster_tmdb_ref,
    DROP COLUMN IF EXISTS poster_copy_checked_at,
    DROP COLUMN IF EXISTS poster_copy_fetched_at,
    DROP COLUMN IF EXISTS poster_copy_next_attempt_at,
    DROP COLUMN IF EXISTS poster_copy_attempts,
    DROP COLUMN IF EXISTS poster_copy_error,
    DROP COLUMN IF EXISTS poster_copy_status,
    DROP COLUMN IF EXISTS poster_copy_path,
    DROP COLUMN IF EXISTS poster_copy_source_url;
