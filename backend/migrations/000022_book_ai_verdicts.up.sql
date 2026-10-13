-- AI pre-moderation verdicts (epic «Автоматическое наполнение каталога»,
-- openspec change add-ai-book-premoderation). At most one current verdict
-- per book, kept apart from the book's own data: the AI never writes book
-- fields. Deleting a book deletes its verdict; a status change keeps it.
-- reason_codes is a JSON array of strings stored as text. input_hash is the
-- SHA-256 of the book's model input (staleness); token counts are stored
-- for cost auditing.
CREATE TABLE book_ai_verdicts (
    book_id        UUID PRIMARY KEY REFERENCES books(id) ON DELETE CASCADE,
    verdict        VARCHAR(20)  NOT NULL CHECK (verdict IN ('publish', 'reject', 'needs_human')),
    reason_codes   TEXT         NOT NULL DEFAULT '[]',
    reason         TEXT         NOT NULL DEFAULT '',
    confidence     NUMERIC(3,2) NOT NULL DEFAULT 0 CHECK (confidence >= 0 AND confidence <= 1),
    model          VARCHAR(100) NOT NULL,
    prompt_version INTEGER      NOT NULL,
    input_hash     CHAR(64)     NOT NULL,
    input_tokens   INTEGER      NOT NULL DEFAULT 0,
    output_tokens  INTEGER      NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_book_ai_verdicts_verdict ON book_ai_verdicts (verdict);
