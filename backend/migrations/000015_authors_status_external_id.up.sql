ALTER TABLE authors ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'published';
ALTER TABLE authors ADD CONSTRAINT chk_authors_status CHECK (status IN ('pending','published','rejected'));
ALTER TABLE authors ADD COLUMN external_source VARCHAR(50);
ALTER TABLE authors ADD COLUMN external_id VARCHAR(255);
CREATE UNIQUE INDEX idx_authors_external_source_id ON authors(external_source, external_id) WHERE external_source IS NOT NULL;
CREATE INDEX idx_authors_status_not_published ON authors(status) WHERE status <> 'published';
