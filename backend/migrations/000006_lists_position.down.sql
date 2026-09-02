DROP INDEX IF EXISTS idx_list_items_book_id;
ALTER TABLE list_items DROP COLUMN IF EXISTS position;
