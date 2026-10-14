CREATE TABLE curated_collections (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug         VARCHAR(100) NOT NULL UNIQUE,
    title        VARCHAR(255) NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    kind         VARCHAR(20) NOT NULL DEFAULT 'manual' CHECK (kind IN ('manual')),
    size         INTEGER NOT NULL CHECK (size BETWEEN 1 AND 1000),
    source       VARCHAR(255),
    source_url   VARCHAR(500),
    genre_id     UUID REFERENCES genres(id) ON DELETE SET NULL,
    is_published BOOLEAN NOT NULL DEFAULT FALSE,
    published_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_curated_collections_published ON curated_collections (is_published, published_at DESC);

CREATE TABLE curated_collection_items (
    collection_id UUID NOT NULL REFERENCES curated_collections(id) ON DELETE CASCADE,
    book_id       UUID NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    position      INTEGER NOT NULL CHECK (position >= 1),
    added_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (collection_id, book_id)
);
CREATE INDEX idx_curated_collection_items_order ON curated_collection_items (collection_id, position);
CREATE INDEX idx_curated_collection_items_book ON curated_collection_items (book_id);
