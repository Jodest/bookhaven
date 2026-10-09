ALTER TABLE books ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'pending';
ALTER TABLE books ADD CONSTRAINT chk_books_status CHECK (status IN ('pending','published','rejected'));
UPDATE books SET status = 'published' WHERE is_hidden = FALSE;
DROP INDEX IF EXISTS idx_books_is_hidden;
ALTER TABLE books DROP COLUMN is_hidden;
CREATE INDEX idx_books_status_not_published ON books(status) WHERE status <> 'published';
