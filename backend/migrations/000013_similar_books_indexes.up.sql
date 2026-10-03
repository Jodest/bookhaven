CREATE INDEX IF NOT EXISTS idx_book_authors_author_id ON book_authors(author_id);
CREATE INDEX IF NOT EXISTS idx_book_genres_genre_id ON book_genres(genre_id);
CREATE INDEX IF NOT EXISTS idx_lists_user_id ON lists(user_id);
