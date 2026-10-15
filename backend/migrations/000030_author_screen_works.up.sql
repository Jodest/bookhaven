CREATE TABLE author_screen_works (
    author_id      UUID NOT NULL REFERENCES authors(id) ON DELETE CASCADE,
    screen_work_id UUID NOT NULL REFERENCES screen_works(id) ON DELETE CASCADE,
    relation       VARCHAR(20) NOT NULL CHECK (relation IN ('biopic','documentary')),
    hidden         BOOLEAN NOT NULL DEFAULT FALSE,
    source         VARCHAR(20) NOT NULL DEFAULT 'wikidata' CHECK (source IN ('wikidata','manual')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (author_id, screen_work_id)
);
CREATE INDEX idx_author_screen_works_work ON author_screen_works (screen_work_id);
