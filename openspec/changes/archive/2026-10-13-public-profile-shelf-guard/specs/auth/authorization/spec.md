## MODIFIED Requirements

### Requirement: Web page guards
The web app SHALL send a visitor with neither an `access_token` nor a `has_session` cookie from `/profile…` and `/shelves…` to `/login?redirect=<path>`, except the public pages `/profile/{id}` (a reader's public profile) and `/shelves/public/{id}`, which MUST open for everyone; a `has_session` cookie alone lets a guarded page load so the app can refresh the session. A visitor with an `access_token` cookie opening `/login` or `/register` MUST be redirected to `/profile`. `/verify-email`, `/forgot-password` and `/reset-password` MUST stay reachable whether signed in or not.

#### Scenario: Anonymous visitor opens shelves
- **WHEN** a visitor without session cookies opens `/shelves`
- **THEN** they are redirected to `/login?redirect=/shelves`

#### Scenario: Anonymous visitor opens a public profile
- **WHEN** a visitor without session cookies opens `/profile/{id}` of a reader
- **THEN** the reader's public profile is shown without a redirect
