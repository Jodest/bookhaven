ALTER TABLE genres ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'pending';
ALTER TABLE genres ADD CONSTRAINT chk_genres_status CHECK (status IN ('pending','published','rejected'));
ALTER TABLE genres ADD COLUMN wikidata_qid VARCHAR(32);
CREATE UNIQUE INDEX idx_genres_wikidata_qid ON genres(wikidata_qid) WHERE wikidata_qid IS NOT NULL;
CREATE INDEX idx_genres_status_not_published ON genres(status) WHERE status <> 'published';
