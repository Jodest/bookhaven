-- Align every review's rating with the reviewer's current rating (ratings is the source of truth).
UPDATE reviews r
SET rating = ra.value
FROM ratings ra
WHERE ra.book_id = r.book_id
  AND ra.user_id = r.user_id
  AND r.rating <> ra.value;
