DROP INDEX IF EXISTS idx_books_status_not_published;
ALTER TABLE books ADD COLUMN is_hidden BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE books SET is_hidden = (status <> 'published');
CREATE INDEX idx_books_is_hidden ON books(is_hidden);
ALTER TABLE books DROP CONSTRAINT IF EXISTS chk_books_status;
ALTER TABLE books DROP COLUMN status;
