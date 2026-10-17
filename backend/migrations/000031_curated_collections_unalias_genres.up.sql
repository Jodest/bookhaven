-- Data fix: curated collections still linked to a genre merged before merges
-- re-pointed collections follow the merge target.
UPDATE curated_collections c
SET genre_id = g.merged_into_id, updated_at = NOW()
FROM genres g
WHERE c.genre_id = g.id AND g.merged_into_id IS NOT NULL;
