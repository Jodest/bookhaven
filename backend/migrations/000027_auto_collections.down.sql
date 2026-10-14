DROP INDEX IF EXISTS idx_curated_collections_kind;

ALTER TABLE curated_collection_items
    DROP COLUMN IF EXISTS score,
    DROP COLUMN IF EXISTS previous_position;

DELETE FROM curated_collections WHERE kind = 'auto';

ALTER TABLE curated_collections
    DROP CONSTRAINT IF EXISTS curated_collections_auto_rule,
    DROP COLUMN IF EXISTS baseline_at,
    DROP COLUMN IF EXISTS computed_at,
    DROP COLUMN IF EXISTS rule_confidence,
    DROP COLUMN IF EXISTS rule_min_ratings,
    DROP COLUMN IF EXISTS rule_period_days,
    DROP COLUMN IF EXISTS rule_metric;

ALTER TABLE curated_collections DROP CONSTRAINT curated_collections_kind_check;
ALTER TABLE curated_collections ADD CONSTRAINT curated_collections_kind_check CHECK (kind IN ('manual'));
