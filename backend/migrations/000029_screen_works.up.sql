CREATE TABLE screen_works (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title              VARCHAR(500) NOT NULL,
    original_title     VARCHAR(500),
    kind               VARCHAR(30) NOT NULL CHECK (kind IN ('film','tv_film','series','animated_film','animated_series','teleplay')),
    year               INTEGER CHECK (year BETWEEN 1880 AND 2100),
    director           VARCHAR(500),
    imdb_id            VARCHAR(20) CHECK (imdb_id ~ '^tt[0-9]+$'),
    kinopoisk_id       VARCHAR(20) CHECK (kinopoisk_id ~ '^[0-9]+$'),
    external_source    VARCHAR(50),
    external_id        VARCHAR(255),
    manually_edited_at TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK ((external_source IS NULL) = (external_id IS NULL))
);
CREATE UNIQUE INDEX uq_screen_works_external ON screen_works (external_source, external_id) WHERE external_id IS NOT NULL;

CREATE TABLE book_screen_works (
    book_id        UUID NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    screen_work_id UUID NOT NULL REFERENCES screen_works(id) ON DELETE CASCADE,
    hidden         BOOLEAN NOT NULL DEFAULT FALSE,
    source         VARCHAR(20) NOT NULL DEFAULT 'wikidata' CHECK (source IN ('wikidata','manual')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (book_id, screen_work_id)
);
CREATE INDEX idx_book_screen_works_work ON book_screen_works (screen_work_id);
