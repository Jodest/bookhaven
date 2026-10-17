## MODIFIED Requirements

### Requirement: Optional authentication
Endpoints with optional authentication (for example public shelves, book reviews and user profiles) SHALL treat a request without a token, with an invalid token or from an unconfirmed user as anonymous instead of failing.

#### Scenario: Invalid token on a public page
- **WHEN** a request with an expired token reads a public shelf
- **THEN** the shelf is returned as for an anonymous visitor
