## ADDED Requirements

### Requirement: Refresh token reuse detection
Every refresh token SHALL belong to a session family: a login starts a new family, and the token issued by a refresh MUST belong to the family of the token it replaces. Tokens that existed before families were introduced each form their own family. When `POST /api/v1/auth/refresh` receives a token that was revoked because it was rotated, the system MUST decide by the time since that rotation:
- within 30 seconds (grace window) the response MUST be 401 with code `refresh_token_superseded`, and nothing is revoked;
- later than 30 seconds the system MUST revoke every still-active token of that token's family, MUST NOT revoke tokens of the user's other families, MUST log a warning that identifies the user and the family but contains no token value or token hash, and the response MUST be 401 without a code, the same body as for an unknown token.
A token revoked for any other reason (logout, password reset, a refresh rejected for an unconfirmed email, or a family revocation) MUST be answered with the plain 401 and MUST NOT trigger a family revocation. The refresh endpoint stays public (no bearer token, no role); the token itself is the credential.

#### Scenario: Stolen token replayed after rotation
- **WHEN** a refresh token was rotated by its owner, and a minute later someone refreshes with the old token
- **THEN** the response is 401 without a code, the owner's current refresh token of that session stops working, and a reuse warning is logged without any token value

#### Scenario: Other sessions survive a reuse
- **WHEN** a user is logged in on a laptop and a phone, and a rotated token of the laptop session is replayed after the grace window
- **THEN** the laptop session can no longer refresh, while the phone session still refreshes successfully

#### Scenario: Re-presenting a token within the grace window
- **WHEN** a client refreshes with a token that was rotated 5 seconds earlier
- **THEN** the response is 401 with code `refresh_token_superseded`, and the token issued by that rotation still refreshes successfully

#### Scenario: Token revoked by logout is not a reuse
- **WHEN** a user logs out and a client later refreshes with a refresh token of that user that had not been rotated
- **THEN** the response is 401 without a code and no reuse warning is logged

#### Scenario: Existing sessions keep working after the upgrade
- **WHEN** a user holds a refresh token issued before session families existed and refreshes with it
- **THEN** the refresh succeeds, and a replay of that old token after the grace window revokes only the token issued by that refresh

## MODIFIED Requirements

### Requirement: Refresh with rotation
The system SHALL accept `POST /api/v1/auth/refresh` with the refresh token in the body (`refresh_token`) or, when absent, from the cookie; with neither the response is 400. An unknown, expired or revoked token MUST return 401 (a token revoked by rotation is answered as described in "Refresh token reuse detection"). A valid token MUST be revoked and replaced atomically: the old token is revoked only if it is still active at that moment, and the new token exists if and only if the old one was revoked by this refresh. Of several concurrent refreshes with the same token, exactly one MUST succeed; the others are answered as a token rotated within the grace window. On success the response is 200 with a new token pair and new cookies. If the token's user has an unconfirmed email, all of that user's refresh tokens MUST be revoked, the cookies cleared, and the response is 401 with code `email_not_verified`.

#### Scenario: Reusing a rotated token
- **WHEN** a client refreshes with a refresh token that was already used for a refresh more than 30 seconds earlier
- **THEN** the response is 401 without a code and the session that token belonged to is revoked

#### Scenario: Two concurrent refreshes with one token
- **WHEN** two requests refresh with the same valid refresh token at the same moment
- **THEN** exactly one receives 200 with a new token pair, the other receives 401 with code `refresh_token_superseded`, and only one new refresh token exists

#### Scenario: Refresh failing after the old token was revoked
- **WHEN** a refresh revokes the old token but creating the new token fails
- **THEN** the old token is not revoked (the change is rolled back) and the response is an error without new cookies

### Requirement: Keeping the session alive in the apps
The web and admin apps SHALL refresh the access token proactively at about 80% of its lifetime and, as a fallback, refresh once and retry when an authenticated request returns 401. Concurrent refresh attempts MUST be merged into one call, because each refresh rotates the token. When the refresh itself answers 401 with code `refresh_token_superseded` (another tab of the same browser rotated the shared cookie a moment earlier), the app MUST wait about one second and repeat the refresh once; this repeat is part of the same merged refresh attempt and is not a second refresh in the sense below. Login, registration, refresh, logout and the public email/password endpoints MUST NOT trigger a refresh. When the refresh fails, or the retried request fails again, the original error is returned without a second refresh, and a 401 means the user is signed out.

#### Scenario: Expired access token
- **WHEN** a signed-in user's access token has expired and a request returns 401
- **THEN** the app refreshes once and retries the request without the user noticing

#### Scenario: Two tabs refresh at the same moment
- **WHEN** two tabs of the same browser refresh at the same moment (for example after the computer wakes up) and one of them receives 401 with code `refresh_token_superseded`
- **THEN** that tab repeats the refresh once after about a second with the newer cookie, succeeds, and both tabs stay signed in

#### Scenario: Superseded twice
- **WHEN** the repeated refresh also answers 401
- **THEN** the app stops refreshing and treats the user as signed out

### Requirement: Refresh token retention
The system SHALL permanently delete stored refresh tokens that were revoked (by rotation, logout, an unconfirmed-email refresh, a password reset or a family revocation) or that expired, no earlier than 7 days after the revocation or expiry, by a background purge that runs at least once a day. A token that has not expired and has not been revoked MUST never be deleted by the purge. The purge MUST NOT change any response: a deleted token is answered by `POST /api/v1/auth/refresh` exactly like a revoked or expired one (401 without a code). Because a deleted token can no longer be recognised, presenting it MUST NOT trigger reuse detection; this is accepted, since such a token was rotated at least 7 days earlier. The purge has no endpoint and needs no role.

#### Scenario: Long-revoked token
- **WHEN** a client refreshes with a refresh token that was rotated more than 7 days ago and has since been purged
- **THEN** the response is 401, the same as for a revoked token that is still stored

#### Scenario: Active session survives the purge
- **WHEN** the purge runs while a user holds a refresh token that is neither expired nor revoked
- **THEN** the token is kept and the next refresh with it succeeds

#### Scenario: Purged token is not detected as a reuse
- **WHEN** a client refreshes with a rotated refresh token that has already been purged while its session is still active
- **THEN** the response is 401 without a code, the session's current token keeps working and no reuse warning is logged
