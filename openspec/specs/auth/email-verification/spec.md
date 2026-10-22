# auth/email-verification Specification

## Purpose

Confirms that a user owns their email address before the account can be used, with one-time links and an enumeration-safe way to request a new one.

## Requirements

### Requirement: Confirmation is required to use an account
An account whose email is not confirmed SHALL NOT be authenticated anywhere: login answers 403 `email_not_verified` (see `auth/session`), refresh revokes the user's tokens, a request with a still-valid access token MUST get 401 with code `email_not_verified`, and optional-auth endpoints treat the request as anonymous.

#### Scenario: Old access token of an unconfirmed user
- **WHEN** a request carries a valid access token of a user whose email is not confirmed
- **THEN** an authenticated endpoint answers 401 with code `email_not_verified`

### Requirement: Confirmation links
A confirmation link SHALL point to `{web}/verify-email?token=…` with a random one-time token valid for 24 hours; only its hash is stored, and the token and link are never logged. A token is bound to the address it was sent to: if the account's email has changed since, the token is invalid.

#### Scenario: Link lifetime
- **WHEN** a user opens a confirmation link 25 hours after it was sent
- **THEN** the link is reported as expired

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

### Requirement: Resend endpoint
The system SHALL accept `POST /api/v1/auth/resend-verification` with `{email}` without authentication (400 `invalid_email` for an invalid address). For an unconfirmed account with a password it MUST invalidate the account's unused links and send a new one. The response MUST be 202 `{status: "sent"}` for every valid address — unknown, already confirmed, or when sending failed — so the endpoint does not reveal which addresses are registered.

#### Scenario: Unknown address
- **WHEN** someone requests a new link for an address with no account
- **THEN** the response is 202 `sent` and no email is sent

#### Scenario: New link replaces the old one
- **WHEN** a user requests a new link and then opens the previous one
- **THEN** the previous link is reported as invalid

### Requirement: Resend rate limits
The resend endpoint SHALL allow at most 20 requests per hour per client IP, and per address (compared trimmed and case-insensitively) at most one request per minute and 5 per day. The limits MUST be checked before the account lookup, so a limited answer reveals nothing. A limited request MUST return 429 with code `rate_limited`, `retry_after_seconds` (at least 1) and a matching `Retry-After` header. An IP-limited request does not use up the address quota.

#### Scenario: Second request within a minute
- **WHEN** someone requests a new link for the same address twice within a minute
- **THEN** the second response is 429 with `retry_after_seconds`

### Requirement: Verification pages
The web app SHALL provide `/verify-email`, public for signed-in and anonymous visitors, which confirms the token from the link on open («Подтверждаем email…») and shows «Email подтверждён» / «Спасибо! Адрес электронной почты подтверждён.», «Email уже подтверждён», «Ссылка неполная» (no token), «Ссылка недействительна», «Срок действия ссылки истёк» («Ссылка действительна 24 часа. Запросите новое письмо.») or «Ссылка уже использована». Wherever a new link can help, the page offers «Отправить письмо ещё раз». After a resend it shows «Если этот адрес зарегистрирован и не подтверждён, мы отправили на него новое письмо.», and for 429 «Слишком много запросов. Попробуйте снова через …».

#### Scenario: Expired link page
- **WHEN** a visitor opens an expired confirmation link
- **THEN** the page says the link has expired and offers to send a new email

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
