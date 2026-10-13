## MODIFIED Requirements

### Requirement: Roles
Every user SHALL have one role: `user`, `contributor`, `moderator` or `admin`. Admin endpoints MUST require the `admin` role, except review moderation, which allows `admin` and `moderator`. An authenticated user without the required role MUST get 403. `contributor` currently grants nothing beyond `user`.

#### Scenario: Reader calls an admin endpoint
- **WHEN** a user with the role `user` calls `GET /api/v1/admin/books`
- **THEN** the response is 403

#### Scenario: Moderator moderates reviews
- **WHEN** a moderator hides a review through the admin API
- **THEN** the request is allowed

#### Scenario: Contributor calls an admin endpoint
- **WHEN** a user with the role `contributor` calls `GET /api/v1/admin/reviews`
- **THEN** the response is 403
