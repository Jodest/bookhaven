ALTER TABLE list_items ADD COLUMN position INTEGER NOT NULL DEFAULT 0;
CREATE INDEX idx_list_items_book_id ON list_items(book_id);
