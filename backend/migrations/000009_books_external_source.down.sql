DROP INDEX IF EXISTS idx_books_external_source_id;
ALTER TABLE books DROP COLUMN IF EXISTS external_id;
ALTER TABLE books DROP COLUMN IF EXISTS external_source;
