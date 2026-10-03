-- The users.email_verified_at backfill from the up migration is
-- intentionally NOT reverted: it cannot be distinguished from real
-- verifications, and the column itself predates this migration.
DROP TABLE IF EXISTS email_verification_tokens;
