DROP INDEX IF EXISTS idx_authors_status_not_published;
DROP INDEX IF EXISTS idx_authors_external_source_id;
ALTER TABLE authors DROP COLUMN IF EXISTS external_id;
ALTER TABLE authors DROP COLUMN IF EXISTS external_source;
ALTER TABLE authors DROP CONSTRAINT IF EXISTS chk_authors_status;
ALTER TABLE authors DROP COLUMN IF EXISTS status;
