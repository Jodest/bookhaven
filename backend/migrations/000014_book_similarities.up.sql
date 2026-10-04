-- Stage 2 similar books (co-reading + daily precompute), see
-- docs/features/similar-books-stage2.md. Full-replaced daily by the
-- similarity job; a book without rows uses the stage-1 live path.
CREATE TABLE book_similarities (
    book_id         UUID NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    similar_book_id UUID NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    position        SMALLINT NOT NULL,          -- 0-based rank after blend + author cap
    score           DOUBLE PRECISION NOT NULL,  -- final blended score
    content_score   DOUBLE PRECISION NOT NULL,  -- contentNorm (0 for co-only)
    co_score        DOUBLE PRECISION NOT NULL,  -- cosine (0 for content-only)
    co_readers      INTEGER NOT NULL,           -- co(a,b), 0 for content-only; diagnostics only, never exposed
    computed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (book_id, similar_book_id),
    CHECK (book_id <> similar_book_id)
);

-- FK cascade from books(id) via similar_book_id would otherwise scan the table.
CREATE INDEX idx_book_similarities_similar_book_id ON book_similarities(similar_book_id);
