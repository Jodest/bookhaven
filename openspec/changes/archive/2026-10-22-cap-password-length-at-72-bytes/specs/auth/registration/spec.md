## MODIFIED Requirements

### Requirement: Registration endpoint
The system SHALL accept `POST /api/v1/auth/register` with `{email, password, name}` without authentication. `email` MUST be a valid email address, `password` at least 8 characters and at most 72 bytes in UTF-8 encoding, and `name` at least 2 characters, otherwise the response is 400 and no account is created. A password over 72 bytes MUST never produce a 5xx response. On success the response MUST be 201 `{email, verification_required: true}` and MUST NOT contain tokens or set auth cookies: registration never logs the user in.

#### Scenario: Successful registration
- **WHEN** a visitor registers with a new email, an 8-character password and a name
- **THEN** the response is 201 with `verification_required: true`, and no cookies are set

#### Scenario: Short password
- **WHEN** a visitor registers with a 7-character password
- **THEN** the response is 400 and no account is created

#### Scenario: Password exactly at the byte limit
- **WHEN** a visitor registers with a password of 72 Latin characters, or of 36 Cyrillic letters (72 bytes)
- **THEN** the response is 201 and the visitor can later log in with that password

#### Scenario: Password over the byte limit
- **WHEN** a visitor registers with a password of 73 Latin characters, or of 40 Cyrillic letters (80 bytes, fewer than 72 characters)
- **THEN** the response is 400 and no account is created

### Requirement: Registration page
The web app SHALL provide `/register` with name, email and password fields, validating them like the endpoint («Имя должно быть не короче 2 символов», «Введите корректный email», «Пароль должен быть не короче 8 символов», and for a password over 72 bytes in UTF-8 «Пароль слишком длинный: до 72 латинских или 36 русских букв»). A form that fails these checks MUST NOT be sent. After a successful registration it MUST show a «Проверьте почту» message naming the email instead of logging the user in. A signed-in visitor opening `/register` MUST be redirected to `/profile`.

#### Scenario: After registering
- **WHEN** a visitor submits the registration form successfully
- **THEN** the page tells them to check their mail and confirm the address

#### Scenario: Too long a password on the page
- **WHEN** a visitor enters a password of 40 Cyrillic letters and submits the form
- **THEN** the password field shows «Пароль слишком длинный: до 72 латинских или 36 русских букв» and no request is sent
