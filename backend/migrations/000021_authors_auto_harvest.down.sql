DROP INDEX IF EXISTS idx_authors_auto_harvest_rotation;
ALTER TABLE authors DROP COLUMN IF EXISTS last_auto_harvested_at;
ALTER TABLE authors DROP COLUMN IF EXISTS auto_harvest;
