-- Linked copies go back to pending (the next sweep decides again).
UPDATE books SET cover_status = 'pending', cover_next_attempt_at = NULL WHERE cover_status = 'linked';
ALTER TABLE books DROP CONSTRAINT IF EXISTS books_cover_status_check;
ALTER TABLE books ADD CONSTRAINT books_cover_status_check
    CHECK (cover_status IN ('pending', 'stored', 'failed', 'absent'));
