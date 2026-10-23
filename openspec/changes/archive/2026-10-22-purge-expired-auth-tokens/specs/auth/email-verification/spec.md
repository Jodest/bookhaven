## MODIFIED Requirements

### Requirement: Verify endpoint
The system SHALL accept `POST /api/v1/auth/verify-email` with `{token}` without authentication and without changing cookies. A valid unused token MUST confirm the email and mark the token used; the response is 200 `{status: "verified"}`. If the account is already confirmed, the response MUST be 200 `{status: "already_verified"}`, even for a used or expired token, as long as the token is still retained (see Verification token retention). Otherwise errors are 400 with code `invalid_token` (malformed, unknown — including a token removed after its retention window — or bound to another address), `token_expired` or `token_used`. Two concurrent uses of one token MUST confirm the email once.

#### Scenario: Second click on the link
- **WHEN** a user opens the same confirmation link twice
- **THEN** the first answer is `verified` and the second `already_verified`

#### Scenario: Malformed token
- **WHEN** the token is not 64 hexadecimal characters
- **THEN** the response is 400 with code `invalid_token`

#### Scenario: Very old link after confirmation
- **WHEN** a confirmed user opens a confirmation link that expired more than 30 days ago and has since been purged
- **THEN** the response is 400 with code `invalid_token`

## ADDED Requirements

### Requirement: Verification token retention
Every confirmation token, used or not, SHALL be kept for at least 30 days after its expiry, so that within that window the verify endpoint answers `already_verified`, `token_used` or `token_expired` exactly as for a fresh lookup. After the window, a background purge that runs at least once a day MUST delete it permanently; the link is then unknown and answers `invalid_token`, which the `/verify-email` page shows as «Ссылка недействительна» with «Отправить письмо ещё раз». A token whose expiry has not passed MUST never be deleted by the purge (replacing unused links on resend is unchanged).

#### Scenario: Used link inside the retention window
- **WHEN** a confirmed user opens their already-used confirmation link 10 days after it was sent
- **THEN** the response is 200 `already_verified`

#### Scenario: Expired link inside the retention window
- **WHEN** an unconfirmed user opens a confirmation link 5 days after it was sent
- **THEN** the response is 400 `token_expired`

#### Scenario: Purged link page
- **WHEN** a visitor opens a confirmation link whose token was purged after the retention window
- **THEN** the page shows «Ссылка недействительна» and offers «Отправить письмо ещё раз»
