# auth/password-reset Specification

## Purpose

Lets a user who forgot their password set a new one through a one-time emailed link, without revealing which addresses are registered.

## Requirements

### Requirement: Forgot-password endpoint
The system SHALL accept `POST /api/v1/auth/forgot-password` with `{email}` without authentication (400 `invalid_email` for an invalid address). For an account with a password, confirmed or not, it MUST invalidate the account's unused reset links and send a new link `{web}/reset-password?token=…`, valid for 1 hour and usable once; only the token's hash is stored. The response MUST be 202 `{status: "sent"}` for every valid address, including unknown ones and failed sends.

#### Scenario: Unknown address
- **WHEN** someone requests a reset for an address with no account
- **THEN** the response is 202 `sent` and no email is sent

#### Scenario: Only the newest link works
- **WHEN** a user requests two reset links and opens the first
- **THEN** the first link is reported as invalid

### Requirement: Forgot-password rate limits
The forgot-password endpoint SHALL apply the same limits as the verification resend, with a separate quota: 20 requests per hour per IP, and per address one per minute and 5 per day, checked before the account lookup, answering 429 `rate_limited` with `retry_after_seconds` and `Retry-After`.

#### Scenario: Separate quota
- **WHEN** a user has used up the verification resend quota for an address
- **THEN** a password reset for that address is still allowed

### Requirement: Reset endpoint
The system SHALL accept `POST /api/v1/auth/reset-password` with `{token, password}` without authentication. The new password MUST be checked first (at least 8 characters, otherwise 400 `invalid_password`), so a rejected password does not use up the link. Token errors are 400 `invalid_token` (malformed, unknown, or bound to an address the account no longer has), `token_expired` or `token_used`. On success the response is 200 `{status: "password_reset"}` and the auth cookies are cleared; the user is not logged in.

#### Scenario: Too short a password
- **WHEN** a user submits a 6-character password with a valid link
- **THEN** the response is 400 `invalid_password` and the link still works

### Requirement: Effects of a reset
A successful reset SHALL, in one transaction: mark the token used, replace the password, confirm the email if it was not confirmed (the link proves mailbox ownership), invalidate the user's other unused reset links and revoke all of the user's refresh tokens, ending every session. Two concurrent uses of one link MUST change the password once; the other gets `token_used`.

#### Scenario: Reset ends all sessions
- **WHEN** a user resets the password while logged in on another device
- **THEN** that device can no longer refresh its session

#### Scenario: Reset confirms the email
- **WHEN** a user with an unconfirmed email resets the password
- **THEN** the email is confirmed and the user can log in with the new password

### Requirement: Password reset pages
The web app SHALL provide public `/forgot-password` («Отправить ссылку», then «Если аккаунт с таким email существует, мы отправили на него письмо со ссылкой для сброса пароля. Ссылка действительна 1 час.», with «Отправить ещё раз») and `/reset-password` (new password and its repetition, «Пароли не совпадают», «Сохранить пароль»). On success it shows «Пароль изменён» / «Теперь войдите с новым паролем. Все прежние сеансы завершены.» with a link to log in. Token errors show «Ссылка неполная», «Ссылка недействительна», «Срок действия ссылки истёк» («Ссылка действительна 1 час. Запросите новую.») or «Ссылка уже использована», each with «Запросить новую ссылку». Neither page redirects a signed-in user.

#### Scenario: Used link
- **WHEN** a visitor opens a reset link that was already used
- **THEN** the page says the password was already changed with this link and offers to request a new one
