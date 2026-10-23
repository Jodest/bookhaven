## MODIFIED Requirements

### Requirement: Effects of a hidden review
A hidden review SHALL NOT appear in the book's public review list or on the home page. Nobody can like it (404). Its author cannot edit it (409 «review is hidden»), but MUST be able to delete it. Deleting keeps the author's rating and removes the review's likes and notifications, exactly like deleting a visible review. While the hidden review exists, its author cannot write a new review of the book (409); after deleting it, they can. The author still receives it as `my_review` when listing the book's reviews, marked with `is_hidden: true`, and the book page shows it to them as hidden.

#### Scenario: Restoring brings it back
- **WHEN** a moderator restores a hidden review
- **THEN** it appears again in the book's public reviews and counts in `reviews_count`

#### Scenario: Author deletes the hidden review
- **WHEN** the author deletes their hidden review
- **THEN** it disappears from the moderation list, the author's rating still counts in the book's average, and the author can write a new review of the book
