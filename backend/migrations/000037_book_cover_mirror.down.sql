DROP INDEX IF EXISTS idx_books_cover_path;
DROP INDEX IF EXISTS idx_books_cover_due;

ALTER TABLE books
    DROP COLUMN IF EXISTS cover_checked_at,
    DROP COLUMN IF EXISTS cover_stored_at,
    DROP COLUMN IF EXISTS cover_next_attempt_at,
    DROP COLUMN IF EXISTS cover_attempts,
    DROP COLUMN IF EXISTS cover_error,
    DROP COLUMN IF EXISTS cover_status,
    DROP COLUMN IF EXISTS cover_path,
    DROP COLUMN IF EXISTS cover_source_url;
