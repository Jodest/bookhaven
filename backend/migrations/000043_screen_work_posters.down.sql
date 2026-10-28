DROP INDEX IF EXISTS idx_screen_works_poster_path;

ALTER TABLE screen_works
    DROP CONSTRAINT IF EXISTS chk_screen_works_poster,
    DROP COLUMN IF EXISTS poster_updated_at,
    DROP COLUMN IF EXISTS poster_source_url,
    DROP COLUMN IF EXISTS poster_license_url,
    DROP COLUMN IF EXISTS poster_license,
    DROP COLUMN IF EXISTS poster_author,
    DROP COLUMN IF EXISTS poster_path,
    DROP COLUMN IF EXISTS poster_url,
    DROP COLUMN IF EXISTS poster_source;
