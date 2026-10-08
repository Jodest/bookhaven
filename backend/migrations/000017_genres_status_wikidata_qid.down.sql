DROP INDEX IF EXISTS idx_genres_status_not_published;
DROP INDEX IF EXISTS idx_genres_wikidata_qid;
ALTER TABLE genres DROP COLUMN IF EXISTS wikidata_qid;
ALTER TABLE genres DROP CONSTRAINT IF EXISTS chk_genres_status;
ALTER TABLE genres DROP COLUMN IF EXISTS status;
