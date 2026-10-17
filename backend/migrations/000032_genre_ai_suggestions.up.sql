CREATE TABLE genre_ai_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trigger VARCHAR(20) NOT NULL CHECK (trigger IN ('manual','scheduled')),
    status VARCHAR(20) NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','done','failed')),
    error_code VARCHAR(40),
    model VARCHAR(100) NOT NULL DEFAULT '',
    prompt_version INTEGER NOT NULL,
    fingerprint CHAR(64) NOT NULL DEFAULT '',
    batches_total INTEGER NOT NULL DEFAULT 0,
    batches_done INTEGER NOT NULL DEFAULT 0,
    suggestions_created INTEGER NOT NULL DEFAULT 0,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);
CREATE INDEX idx_genre_ai_runs_created_at ON genre_ai_runs (created_at DESC);

CREATE TABLE genre_ai_suggestions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID REFERENCES genre_ai_runs(id) ON DELETE SET NULL,
    kind VARCHAR(20) NOT NULL CHECK (kind IN ('duplicate','narrow','junk')),
    status VARCHAR(20) NOT NULL DEFAULT 'open' CHECK (status IN ('open','applied','dismissed','stale')),
    set_key CHAR(64) NOT NULL,
    reason_codes TEXT NOT NULL DEFAULT '[]',
    reason TEXT NOT NULL DEFAULT '',
    confidence NUMERIC(3,2) NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    model VARCHAR(100) NOT NULL,
    prompt_version INTEGER NOT NULL,
    applied_target_id UUID,
    decided_by UUID REFERENCES users(id) ON DELETE SET NULL,
    decided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_genre_ai_suggestions_set_key_live ON genre_ai_suggestions (set_key) WHERE status IN ('open','dismissed');
CREATE INDEX idx_genre_ai_suggestions_status ON genre_ai_suggestions (status, kind, confidence DESC, created_at DESC);

CREATE TABLE genre_ai_suggestion_genres (
    suggestion_id UUID NOT NULL REFERENCES genre_ai_suggestions(id) ON DELETE CASCADE,
    genre_id UUID NOT NULL,
    role VARCHAR(10) NOT NULL CHECK (role IN ('target','source')),
    position INTEGER NOT NULL,
    name_snapshot VARCHAR(255) NOT NULL,
    PRIMARY KEY (suggestion_id, genre_id)
);
CREATE INDEX idx_genre_ai_suggestion_genres_genre ON genre_ai_suggestion_genres (genre_id);
