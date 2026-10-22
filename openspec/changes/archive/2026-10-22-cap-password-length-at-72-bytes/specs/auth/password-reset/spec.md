## MODIFIED Requirements

### Requirement: Reset endpoint
The system SHALL accept `POST /api/v1/auth/reset-password` with `{token, password}` without authentication. The new password MUST be checked first (at least 8 characters and at most 72 bytes in UTF-8 encoding, otherwise 400 `invalid_password`), so a rejected password does not use up the link; a password over 72 bytes MUST never produce a 5xx response. Token errors are 400 `invalid_token` (malformed, unknown, or bound to an address the account no longer has), `token_expired` or `token_used`. On success the response is 200 `{status: "password_reset"}` and the auth cookies are cleared; the user is not logged in.

#### Scenario: Too short a password
- **WHEN** a user submits a 6-character password with a valid link
- **THEN** the response is 400 `invalid_password` and the link still works

#### Scenario: Too long a password
- **WHEN** a user submits a password of 40 Cyrillic letters (80 bytes) with a valid link
- **THEN** the response is 400 `invalid_password`, the password is unchanged and the link still works

#### Scenario: Password exactly at the byte limit
- **WHEN** a user submits a password of 36 Cyrillic letters (72 bytes) with a valid link
- **THEN** the response is 200 `password_reset` and the user can log in with the new password

### Requirement: Password reset pages
The web app SHALL provide public `/forgot-password` («Отправить ссылку», then «Если аккаунт с таким email существует, мы отправили на него письмо со ссылкой для сброса пароля. Ссылка действительна 1 час.», with «Отправить ещё раз») and `/reset-password` (new password and its repetition, «Пароли не совпадают», «Сохранить пароль»). The new password MUST be checked like the endpoint before sending: «Пароль должен быть не короче 8 символов», and for a password over 72 bytes in UTF-8 «Пароль слишком длинный: до 72 латинских или 36 русских букв». On success it shows «Пароль изменён» / «Теперь войдите с новым паролем. Все прежние сеансы завершены.» with a link to log in. Token errors show «Ссылка неполная», «Ссылка недействительна», «Срок действия ссылки истёк» («Ссылка действительна 1 час. Запросите новую.») or «Ссылка уже использована», each with «Запросить новую ссылку». Neither page redirects a signed-in user.

#### Scenario: Used link
- **WHEN** a visitor opens a reset link that was already used
- **THEN** the page says the password was already changed with this link and offers to request a new one

#### Scenario: Too long a new password on the page
- **WHEN** a visitor enters the same 40-Cyrillic-letter password in both fields and submits
- **THEN** the password field shows «Пароль слишком длинный: до 72 латинских или 36 русских букв» and no request is sent
