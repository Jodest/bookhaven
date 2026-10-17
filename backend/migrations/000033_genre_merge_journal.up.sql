CREATE TABLE genre_merges (
    id UUID PRIMARY KEY,
    target_id UUID NOT NULL,
    target_name VARCHAR(255) NOT NULL,
    books_reassigned INTEGER NOT NULL,
    collections_switched INTEGER NOT NULL,
    performed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    undone_at TIMESTAMPTZ,
    undone_by UUID REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX idx_genre_merges_created_at ON genre_merges (created_at DESC, id);
CREATE INDEX idx_genre_merges_target_active ON genre_merges (target_id) WHERE undone_at IS NULL;

CREATE TABLE genre_merge_sources (
    merge_id UUID NOT NULL REFERENCES genre_merges(id) ON DELETE CASCADE,
    source_id UUID NOT NULL,
    position INTEGER NOT NULL,
    source_name VARCHAR(255) NOT NULL,
    prev_status VARCHAR(20) NOT NULL,
    prev_qid VARCHAR(32),
    qid_carried BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (merge_id, source_id)
);

CREATE TABLE genre_merge_items (
    merge_id UUID NOT NULL,
    source_id UUID NOT NULL,
    kind VARCHAR(20) NOT NULL CHECK (kind IN ('book_moved', 'book_linked', 'alias', 'collection')),
    ref_id UUID NOT NULL,
    PRIMARY KEY (merge_id, source_id, kind, ref_id),
    FOREIGN KEY (merge_id, source_id) REFERENCES genre_merge_sources (merge_id, source_id) ON DELETE CASCADE
);
CREATE INDEX idx_genre_merge_items_ref ON genre_merge_items (ref_id);
