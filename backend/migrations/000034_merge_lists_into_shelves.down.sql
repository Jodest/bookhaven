-- DATA LOSS on down: this restores the table structure only.
-- Former lists stay as plain custom shelves (without description or privacy),
-- the recreated lists / list_items tables are empty, and private shelves become
-- public again. Do not run this against a database whose data you want to keep.
ALTER TABLE shelves DROP CONSTRAINT shelves_default_public_chk;
ALTER TABLE shelves DROP COLUMN is_public;
ALTER TABLE shelves DROP COLUMN description;

CREATE TABLE lists (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    is_public BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE list_items (
    list_id UUID NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    book_id UUID NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (list_id, book_id)
);

CREATE INDEX idx_list_items_list_id ON list_items(list_id);
CREATE INDEX idx_list_items_book_id ON list_items(book_id);
CREATE INDEX idx_lists_user_id ON lists(user_id);
