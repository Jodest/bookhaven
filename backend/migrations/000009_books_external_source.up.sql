ALTER TABLE books ADD COLUMN external_source VARCHAR(50);
ALTER TABLE books ADD COLUMN external_id VARCHAR(255);
CREATE UNIQUE INDEX idx_books_external_source_id ON books(external_source, external_id) WHERE external_source IS NOT NULL;
