ALTER TABLE curated_collection_items
    DROP COLUMN IF EXISTS source_share,
    DROP COLUMN IF EXISTS source_votes,
    DROP COLUMN IF EXISTS source_screen_work_id;

ALTER TABLE curated_collections DROP CONSTRAINT IF EXISTS curated_collections_rule_metric_check;
ALTER TABLE curated_collections ADD CONSTRAINT curated_collections_rule_metric_check
    CHECK (rule_metric IN ('weighted_rating', 'most_read', 'most_wanted', 'trending'));
