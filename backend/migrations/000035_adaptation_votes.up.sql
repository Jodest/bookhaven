CREATE TABLE adaptation_votes (
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    book_id        UUID NOT NULL,
    screen_work_id UUID NOT NULL,
    value          VARCHAR(20) NOT NULL CHECK (value IN ('book','adaptation','same')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, book_id, screen_work_id),
    FOREIGN KEY (book_id, screen_work_id)
        REFERENCES book_screen_works (book_id, screen_work_id) ON DELETE CASCADE
);

CREATE INDEX idx_adaptation_votes_link ON adaptation_votes (book_id, screen_work_id);
CREATE INDEX idx_adaptation_votes_updated ON adaptation_votes (updated_at);
