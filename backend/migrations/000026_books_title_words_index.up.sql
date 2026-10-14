-- Word index on normalized titles for bulk-paste matching in curated
-- collections. The query must use this exact expression.
CREATE INDEX IF NOT EXISTS idx_books_title_words
    ON books USING GIN (to_tsvector('simple', replace(lower(title), 'ё', 'е')));
