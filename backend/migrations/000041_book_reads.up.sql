-- Reading journal: shelf = current status, journal = history of reads.
CREATE TABLE book_reads (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    book_id      UUID NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    started_at   TIMESTAMPTZ NULL,
    finished_at  TIMESTAMPTZ NULL,
    abandoned_at TIMESTAMPTZ NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT book_reads_single_outcome CHECK (finished_at IS NULL OR abandoned_at IS NULL)
);

-- at most one open attempt per user and book
CREATE UNIQUE INDEX uq_book_reads_open ON book_reads (user_id, book_id)
    WHERE finished_at IS NULL AND abandoned_at IS NULL;

-- reads_count / last_read_at / IsReader / undo
CREATE INDEX idx_book_reads_user_book_finished ON book_reads (user_id, book_id, finished_at DESC)
    WHERE finished_at IS NOT NULL;

-- challenge count by year
CREATE INDEX idx_book_reads_user_finished ON book_reads (user_id, finished_at)
    WHERE finished_at IS NOT NULL;

-- Backfill from the current shelves.
INSERT INTO book_reads (user_id, book_id, finished_at, created_at)
SELECT s.user_id, si.book_id, si.added_at, si.added_at
FROM shelf_items si JOIN shelves s ON s.id = si.shelf_id WHERE s.type = 'read';

INSERT INTO book_reads (user_id, book_id, started_at, created_at)
SELECT s.user_id, si.book_id, si.added_at, si.added_at
FROM shelf_items si JOIN shelves s ON s.id = si.shelf_id WHERE s.type = 'reading';

INSERT INTO book_reads (user_id, book_id, abandoned_at, created_at)
SELECT s.user_id, si.book_id, si.added_at, si.added_at
FROM shelf_items si JOIN shelves s ON s.id = si.shelf_id WHERE s.type = 'abandoned';
