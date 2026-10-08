ALTER TABLE genres ADD COLUMN merged_into_id UUID REFERENCES genres(id) ON DELETE RESTRICT;
ALTER TABLE genres ADD CONSTRAINT chk_genres_merged_into_not_self CHECK (merged_into_id IS NULL OR merged_into_id <> id);
ALTER TABLE genres ADD CONSTRAINT chk_genres_alias_rejected CHECK (merged_into_id IS NULL OR status = 'rejected');
CREATE INDEX idx_genres_merged_into_id ON genres(merged_into_id) WHERE merged_into_id IS NOT NULL;
