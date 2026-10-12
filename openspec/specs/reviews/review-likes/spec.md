# reviews/review-likes Specification

## Purpose

Lets signed-in readers like reviews, so useful reviews collect visible appreciation and their authors are notified.

## Requirements

### Requirement: Toggle a like
`POST /api/v1/reviews/{id}/likes` SHALL require authentication and toggle the caller's like on the review: it adds the like when there is none and removes it otherwise. The response is 200 `{likes_count, liked}` with the review's new like count and the caller's new state. A user has at most one like per review. An unknown or hidden review MUST return 404. Authors may like their own review.

#### Scenario: Like and unlike
- **WHEN** a reader likes a review with 2 likes and then calls the endpoint again
- **THEN** the first response is `{likes_count: 3, liked: true}` and the second `{likes_count: 2, liked: false}`

#### Scenario: Hidden review
- **WHEN** a reader tries to like a hidden review
- **THEN** the response is 404

### Requirement: Like count
A review's `likes_count` SHALL equal its number of likes, recalculated in the same transaction as each toggle.

#### Scenario: Count follows the likes
- **WHEN** three readers like a review and one of them removes the like
- **THEN** the review's `likes_count` is 2

### Requirement: Liking from the book page
Each review on the book page SHALL show its like count and whether the signed-in reader liked it; clicking toggles the like and updates the count. An anonymous visitor cannot like. A failure shows «Ошибка лайка».

#### Scenario: Liking a review on the page
- **WHEN** a signed-in reader clicks the like button of a review
- **THEN** the count increases by one and the button shows the review as liked
