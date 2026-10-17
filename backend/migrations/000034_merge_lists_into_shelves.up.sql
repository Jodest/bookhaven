-- Merge user lists into custom shelves (openspec change merge-lists-into-shelves).
-- List ids are reused as shelf ids, so list_items.list_id maps straight to
-- shelf_items.shelf_id. A PK collision aborts the migration instead of merging data.
ALTER TABLE shelves ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE shelves ADD COLUMN is_public BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE shelves ADD CONSTRAINT shelves_default_public_chk CHECK (is_public OR type = 'custom');

INSERT INTO shelves (id, user_id, name, type, description, is_public, created_at, updated_at)
SELECT l.id, l.user_id,
       CASE WHEN char_length(n.name) >= 2 THEN n.name ELSE 'Список' END,
       'custom', COALESCE(l.description, ''), l.is_public, l.created_at, l.updated_at
FROM lists l
CROSS JOIN LATERAL (SELECT regexp_replace(LEFT(btrim(l.title), 100), '\s+$', '') AS name) n;

INSERT INTO shelf_items (shelf_id, book_id, added_at)
SELECT list_id, book_id, added_at FROM list_items
ON CONFLICT DO NOTHING;

DROP TABLE list_items;
DROP TABLE lists;
