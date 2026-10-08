DROP INDEX IF EXISTS idx_genres_merged_into_id;
ALTER TABLE genres DROP CONSTRAINT IF EXISTS chk_genres_alias_rejected;
ALTER TABLE genres DROP CONSTRAINT IF EXISTS chk_genres_merged_into_not_self;
ALTER TABLE genres DROP COLUMN IF EXISTS merged_into_id;
