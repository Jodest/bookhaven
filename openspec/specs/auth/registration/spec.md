# auth/registration Specification

## Purpose

Lets a visitor create a reader account with an email and a password. The account cannot be used until the email is confirmed (see `auth/email-verification`).

## Requirements

### Requirement: Registration endpoint
The system SHALL accept `POST /api/v1/auth/register` with `{email, password, name}` without authentication. `email` MUST be a valid email address, `password` at least 8 characters and `name` at least 2 characters, otherwise the response is 400. On success the response MUST be 201 `{email, verification_required: true}` and MUST NOT contain tokens or set auth cookies: registration never logs the user in.

#### Scenario: Successful registration
- **WHEN** a visitor registers with a new email, an 8-character password and a name
- **THEN** the response is 201 with `verification_required: true`, and no cookies are set

#### Scenario: Short password
- **WHEN** a visitor registers with a 7-character password
- **THEN** the response is 400 and no account is created

### Requirement: Unique email
An email SHALL belong to at most one account. Registering an email that already has an account MUST return 409 and MUST NOT change the existing account. Emails are compared exactly as typed.

#### Scenario: Existing email
- **WHEN** a visitor registers with the email of an existing account
- **THEN** the response is 409 and the existing account is unchanged

### Requirement: New account defaults
A registered account SHALL get the role `user`, the reader profile type, a password hash (never the plain password) and an unconfirmed email. Roles above `user` are only assigned by an admin.

#### Scenario: Role of a new account
- **WHEN** a visitor registers
- **THEN** the account has the role `user` and its email is not confirmed

### Requirement: Verification email on registration
Registration SHALL send an email with a confirmation link (see `auth/email-verification`). A failure to create or send that email MUST NOT fail the registration: the account is still created and the user can request a new link.

#### Scenario: Mail queue unavailable
- **WHEN** a visitor registers while the verification email cannot be queued
- **THEN** the response is still 201 and the account exists

### Requirement: Registration page
The web app SHALL provide `/register` with name, email and password fields, validating them like the endpoint («Имя должно быть не короче 2 символов», «Введите корректный email», «Пароль должен быть не короче 8 символов»). After a successful registration it MUST show a «Проверьте почту» message naming the email instead of logging the user in. A signed-in visitor opening `/register` MUST be redirected to `/profile`.

#### Scenario: After registering
- **WHEN** a visitor submits the registration form successfully
- **THEN** the page tells them to check their mail and confirm the address
