-- AI-generated book description drafts (openspec capability
-- catalog-admin/ai-descriptions). Kept apart from books.description: an
-- AI text is never copied into the book's own description, and it is
-- public only when approved and the own description is blank.
CREATE TABLE book_ai_descriptions (
    book_id        UUID PRIMARY KEY REFERENCES books(id) ON DELETE CASCADE,
    status         VARCHAR(20)  NOT NULL CHECK (status IN ('draft','approved','discarded','unknown','failed')),
    text           TEXT         NOT NULL DEFAULT '',
    error_code     VARCHAR(40),
    edited         BOOLEAN      NOT NULL DEFAULT false,
    model          VARCHAR(100) NOT NULL,
    prompt_version INTEGER      NOT NULL,
    input_hash     CHAR(64)     NOT NULL,
    input_tokens   INTEGER      NOT NULL DEFAULT 0,
    output_tokens  INTEGER      NOT NULL DEFAULT 0,
    generated_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    reviewed_by    UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at    TIMESTAMPTZ
);

CREATE INDEX idx_book_ai_descriptions_status ON book_ai_descriptions (status, generated_at DESC);
