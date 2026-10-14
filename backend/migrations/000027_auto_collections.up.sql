ALTER TABLE curated_collections DROP CONSTRAINT curated_collections_kind_check;

ALTER TABLE curated_collections
    ADD CONSTRAINT curated_collections_kind_check CHECK (kind IN ('manual', 'auto')),
    ADD COLUMN rule_metric      VARCHAR(30)
        CHECK (rule_metric IN ('weighted_rating', 'most_read', 'most_wanted', 'trending')),
    ADD COLUMN rule_period_days INTEGER CHECK (rule_period_days BETWEEN 1 AND 3650),
    ADD COLUMN rule_min_ratings INTEGER CHECK (rule_min_ratings BETWEEN 1 AND 10000),
    ADD COLUMN rule_confidence  INTEGER CHECK (rule_confidence BETWEEN 1 AND 10000),
    ADD COLUMN computed_at      TIMESTAMPTZ,
    ADD COLUMN baseline_at      TIMESTAMPTZ,
    ADD CONSTRAINT curated_collections_auto_rule CHECK ((kind = 'auto') = (rule_metric IS NOT NULL));

ALTER TABLE curated_collection_items
    ADD COLUMN previous_position INTEGER CHECK (previous_position >= 1),
    ADD COLUMN score             DOUBLE PRECISION;

CREATE INDEX idx_curated_collections_kind ON curated_collections (kind);
