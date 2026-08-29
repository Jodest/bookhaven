ALTER TABLE shelves DROP CONSTRAINT shelves_user_id_type_key;

CREATE UNIQUE INDEX idx_shelves_user_id_type ON shelves(user_id, type) WHERE type <> 'custom';
CREATE INDEX idx_shelf_items_book_id ON shelf_items(book_id);
