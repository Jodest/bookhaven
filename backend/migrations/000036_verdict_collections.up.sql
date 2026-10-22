-- Verdict-based automatic collections (book_beats_adaptation, adaptation_beats_book).
ALTER TABLE curated_collections DROP CONSTRAINT curated_collections_rule_metric_check;
ALTER TABLE curated_collections ADD CONSTRAINT curated_collections_rule_metric_check
    CHECK (rule_metric IN ('weighted_rating', 'most_read', 'most_wanted', 'trending',
                           'book_beats_adaptation', 'adaptation_beats_book'));

-- The source adaptation a verdict snapshot row was scored by.
ALTER TABLE curated_collection_items
    ADD COLUMN source_screen_work_id UUID REFERENCES screen_works(id) ON DELETE SET NULL,
    ADD COLUMN source_votes          INTEGER CHECK (source_votes >= 1),
    ADD COLUMN source_share          DOUBLE PRECISION CHECK (source_share BETWEEN 0 AND 1);
