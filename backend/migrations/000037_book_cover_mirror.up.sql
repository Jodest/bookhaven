-- Self-hosted covers (#489): books.cover_url stays the source link; these
-- columns describe the local copy made from cover_source_url. Written only by
-- the covers module (raw guarded UPDATE), read-only for GORM on books.
ALTER TABLE books
    ADD COLUMN cover_source_url      TEXT,
    ADD COLUMN cover_path            VARCHAR(80),
    ADD COLUMN cover_status          VARCHAR(16)
        CHECK (cover_status IN ('pending', 'stored', 'failed', 'absent')),
    ADD COLUMN cover_error           VARCHAR(32),
    ADD COLUMN cover_attempts        INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN cover_next_attempt_at TIMESTAMPTZ,
    ADD COLUMN cover_stored_at       TIMESTAMPTZ,
    ADD COLUMN cover_checked_at      TIMESTAMPTZ;

CREATE INDEX idx_books_cover_due ON books (cover_next_attempt_at)
    WHERE cover_url IS NOT NULL AND cover_url <> '';
CREATE INDEX idx_books_cover_path ON books (cover_path)
    WHERE cover_path IS NOT NULL;
