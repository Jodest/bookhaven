DROP INDEX IF EXISTS idx_books_search;
CREATE INDEX idx_books_search ON books USING GIN (to_tsvector('russian', title || ' ' || COALESCE(isbn, '') || ' ' || COALESCE(description, '')));
