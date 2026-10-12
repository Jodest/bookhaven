DROP INDEX IF EXISTS idx_books_published_at;
ALTER TABLE books DROP COLUMN IF EXISTS published_at;
