-- One-time email verification tokens (#158). Only the sha256 hex of the
-- token is stored; the plain token exists only in the emailed link.
-- `email` binds the token to the address it was sent to.
CREATE TABLE email_verification_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_email_verification_tokens_user_id ON email_verification_tokens(user_id);

-- Backfill: every account that existed before email verification was
-- introduced is treated as verified.
UPDATE users SET email_verified_at = now() WHERE email_verified_at IS NULL;
