-- Link-only covers (#686): a book cover on a link-only host (FantLab) is shown
-- by its source link and never downloaded, so its copy state is 'linked'.
ALTER TABLE books DROP CONSTRAINT IF EXISTS books_cover_status_check;
ALTER TABLE books ADD CONSTRAINT books_cover_status_check
    CHECK (cover_status IN ('pending', 'stored', 'failed', 'absent', 'linked'));
