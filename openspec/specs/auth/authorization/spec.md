# auth/authorization Specification

## Purpose

Defines how API requests are authenticated (bearer header or cookie), how roles gate endpoints, why state-changing admin requests require a bearer header, and how the web and admin apps guard their pages.

## Requirements

### Requirement: Authenticated requests
An endpoint that requires authentication SHALL accept the access token from an `Authorization: Bearer` header or, when there is none, from the `access_token` cookie. A missing, invalid, expired or non-access token, or a token of a user that no longer exists, MUST return 401. A user with an unconfirmed email MUST get 401 with code `email_not_verified`.

#### Scenario: Cookie-only request
- **WHEN** a signed-in browser calls `GET /api/v1/auth/me` with only the `access_token` cookie
- **THEN** the request is authenticated

### Requirement: Optional authentication
Endpoints with optional authentication (for example public shelves and lists, book reviews, user profiles) SHALL treat a request without a token, with an invalid token or from an unconfirmed user as anonymous instead of failing.

#### Scenario: Invalid token on a public page
- **WHEN** a request with an expired token reads a public shelf
- **THEN** the shelf is returned as for an anonymous visitor

### Requirement: Roles
Every user SHALL have one role: `user`, `moderator` or `admin`. Admin endpoints MUST require the `admin` role, except review moderation, which allows `admin` and `moderator`. An authenticated user without the required role MUST get 403.

#### Scenario: Reader calls an admin endpoint
- **WHEN** a user with the role `user` calls `GET /api/v1/admin/books`
- **THEN** the response is 403

#### Scenario: Moderator moderates reviews
- **WHEN** a moderator hides a review through the admin API
- **THEN** the request is allowed

### Requirement: Bearer header for state-changing admin requests
State-changing admin requests SHALL require the access token in an `Authorization: Bearer` header, in addition to authentication and role; a cookie-only request MUST get 401 «bearer token required» (or «invalid authorization header» for a malformed header). This protects cookie sessions against cross-site request forgery. Admin reads MAY use the cookie alone. The admin app MUST send the bearer header on its non-GET admin requests, keeping the current access token across page reloads.

#### Scenario: Cookie-only admin write
- **WHEN** an admin's browser sends `POST /api/v1/admin/books/bulk-status` with only the cookie
- **THEN** the response is 401 and nothing changes

### Requirement: Web page guards
The web app SHALL send a visitor with neither an `access_token` nor a `has_session` cookie from `/profile…` and `/shelves…` to `/login?redirect=<path>`; a `has_session` cookie alone lets the page load so the app can refresh the session. A visitor with an `access_token` cookie opening `/login` or `/register` MUST be redirected to `/profile`. `/verify-email`, `/forgot-password` and `/reset-password` MUST stay reachable whether signed in or not.

#### Scenario: Anonymous visitor opens shelves
- **WHEN** a visitor without session cookies opens `/shelves`
- **THEN** they are redirected to `/login?redirect=/shelves`

### Requirement: Admin app access
Every admin page except `/login` SHALL redirect a visitor with neither an `access_token` nor a `has_session` cookie to `/login?redirect=<path>`. Only `admin` and `moderator` users may use the admin app: any other signed-in user MUST see «Доступ запрещен» («У вас недостаточно прав для доступа к панели администратора.»). Sections are shown according to role (a moderator sees review moderation only), and when the server answers 403 to a request on the current page, the page shows that the server rejected the request for lack of rights.

#### Scenario: Reader opens the admin app
- **WHEN** a user with the role `user` signs in to the admin app
- **THEN** the admin app shows «Доступ запрещен»
