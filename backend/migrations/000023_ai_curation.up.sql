-- AI curator of author import lists (epic «Автоматическое наполнение
-- каталога», openspec change add-ai-author-curation). An admin's free-text
-- request and the author names the model proposed for it, each resolved
-- against Wikidata. These rows are not catalog data: nothing here is ever
-- copied into authors/books; the admin takes candidates through the
-- existing import flows. Rows older than 30 days are purged by the worker
-- (candidates cascade). options is a JSON array of {qid,label,description}
-- stored as text. Token counts are stored for cost auditing.
CREATE TABLE ai_curation_requests (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    prompt         TEXT         NOT NULL,
    max_names      INTEGER      NOT NULL CHECK (max_names BETWEEN 1 AND 30),
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
CREATE INDEX idx_ai_curation_requests_created_at ON ai_curation_requests (created_at DESC);
CREATE INDEX idx_ai_curation_requests_creator ON ai_curation_requests (created_by, created_at DESC);

CREATE TABLE ai_curation_candidates (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id   UUID         NOT NULL REFERENCES ai_curation_requests(id) ON DELETE CASCADE,
    position     INTEGER      NOT NULL,
    name         VARCHAR(150) NOT NULL,
    note         VARCHAR(200) NOT NULL DEFAULT '',
    match_status VARCHAR(20)  NOT NULL DEFAULT 'pending'
                 CHECK (match_status IN ('pending', 'matched', 'ambiguous', 'not_found', 'error')),
    options      TEXT         NOT NULL DEFAULT '[]',
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (request_id, position)
);
