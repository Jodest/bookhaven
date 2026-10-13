-- Scheduled harvest of flagged authors (epic «Автоматическое наполнение
-- каталога», openspec change add-scheduled-harvest). auto_harvest is an
-- admin opt-in per author (off for every existing author);
-- last_auto_harvested_at is stamped by the worker's schedule tick after it
-- queued a scheduled harvest job covering the author. The partial index
-- backs the rotation order (never harvested first, then oldest).
ALTER TABLE authors ADD COLUMN auto_harvest BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE authors ADD COLUMN last_auto_harvested_at TIMESTAMPTZ;
CREATE INDEX idx_authors_auto_harvest_rotation
  ON authors (last_auto_harvested_at NULLS FIRST) WHERE auto_harvest;
