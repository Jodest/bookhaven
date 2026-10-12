-- First-publication time of a book (#245). Set once, the first time the book
-- becomes published; never cleared. Existing published books are backfilled
-- with created_at so the home row "Новое в каталоге" keeps its order.
ALTER TABLE books ADD COLUMN published_at TIMESTAMPTZ NULL;
UPDATE books SET published_at = created_at WHERE status = 'published';
CREATE INDEX idx_books_published_at ON books (published_at DESC NULLS LAST, id DESC) WHERE status = 'published';
