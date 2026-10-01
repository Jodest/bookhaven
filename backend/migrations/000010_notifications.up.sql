CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,   -- recipient
    actor_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,  -- who triggered it
    type VARCHAR(32) NOT NULL,                                      -- 'review_like' today
    review_id UUID REFERENCES reviews(id) ON DELETE CASCADE,        -- type-specific payload; nullable so future types that aren't review-related don't need it
    is_read BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (actor_id, review_id, type)
);

CREATE INDEX idx_notifications_user_unread ON notifications(user_id, is_read, created_at DESC);
