DROP INDEX idx_shelf_items_book_id;
DROP INDEX idx_shelves_user_id_type;

ALTER TABLE shelves ADD CONSTRAINT shelves_user_id_type_key UNIQUE (user_id, type);
