-- AI drafts of manual curated collections (epic #413, feature 5, openspec
-- change add-ai-collection-drafts). An admin's brief and the ranked books the
-- model proposed for it. These rows are not catalog data: nothing here is ever
-- copied into books or collections; the admin applies matched catalog books
-- through the existing bulk add. Matching against the catalog is computed at
-- read time. Drafts older than 30 days are purged by the worker (proposals
-- cascade); deleting a collection deletes its drafts. Token counts are stored
-- for cost auditing.
CREATE TABLE ai_collection_drafts (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    collection_id  UUID         NOT NULL REFERENCES curated_collections(id) ON DELETE CASCADE,
    brief          TEXT         NOT NULL,
    count          INTEGER      NOT NULL CHECK (count BETWEEN 1 AND 1000),
    status         VARCHAR(20)  NOT NULL DEFAULT 'queued'
                   CHECK (status IN ('queued', 'running', 'done', 'failed')),
    error_code     VARCHAR(40),
    model          VARCHAR(100) NOT NULL DEFAULT '',
    prompt_version INTEGER      NOT NULL,
    input_tokens   INTEGER      NOT NULL DEFAULT 0,
    output_tokens  INTEGER      NOT NULL DEFAULT 0,
    created_by     UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    finished_at    TIMESTAMPTZ
);

CREATE INDEX idx_ai_collection_drafts_collection ON ai_collection_drafts (collection_id, created_at DESC);
CREATE INDEX idx_ai_collection_drafts_creator ON ai_collection_drafts (created_by, created_at DESC);
CREATE INDEX idx_ai_collection_drafts_created_at ON ai_collection_drafts (created_at);

CREATE TABLE ai_collection_proposals (
    draft_id       UUID         NOT NULL REFERENCES ai_collection_drafts(id) ON DELETE CASCADE,
    rank           INTEGER      NOT NULL CHECK (rank >= 1),
    title          VARCHAR(300) NOT NULL,
    author         VARCHAR(200) NOT NULL DEFAULT '',
    original_title VARCHAR(300) NOT NULL DEFAULT '',
    note           VARCHAR(150) NOT NULL DEFAULT '',
    PRIMARY KEY (draft_id, rank)
);
