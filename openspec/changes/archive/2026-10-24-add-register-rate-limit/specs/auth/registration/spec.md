## ADDED Requirements

### Requirement: Registration rate limits
`POST /api/v1/auth/register` SHALL throttle requests with two counters over fixed one-hour windows: at most 10 requests per client IP and at most 5 requests per submitted email address (compared case-insensitively, ignoring surrounding spaces). Every request that passes request validation counts against both counters, whatever its outcome: a created account (201) and an already registered address (409) count the same. Both limits MUST be checked before the account is looked up and depend only on the request, so a limited response is identical for new and existing addresses. When a limit is reached, the response MUST be 429 with code `rate_limited`, a `retry_after_seconds` body field (at least 1) and a matching `Retry-After` header equal to the time left in the window; no account is created and no email is sent. A request refused by the per-IP limit MUST NOT use up the per-address quota; a request refused by the per-address limit still counts against its IP. A request that fails request validation (400) MUST NOT count against either counter. A request that fails with an internal error MUST NOT count against either counter. The client IP is derived exactly as for login. When the limit store is unavailable, registration MUST keep working without limits rather than fail. The endpoint stays public (no bearer token).

#### Scenario: Too many registrations from one IP
- **WHEN** one client IP sends 10 valid registration requests for different addresses within an hour and then an 11th
- **THEN** the 11th response is 429 `rate_limited` with `retry_after_seconds` and `Retry-After`, no account is created and no verification email is sent

#### Scenario: Too many registrations for one address
- **WHEN** 5 valid registration requests for one address arrive within an hour from different IPs, and then a 6th written in a different case and with surrounding spaces
- **THEN** the 6th response is 429 `rate_limited`

#### Scenario: Limited response does not reveal the account
- **WHEN** an address with an account and an address without one each reach the per-address limit
- **THEN** both receive the same 429 `rate_limited` response

#### Scenario: Existing address counts like a new one
- **WHEN** a client IP gets 10 responses of 409 for already registered addresses within an hour and then registers a new address
- **THEN** the response is 429 `rate_limited` and no account is created

#### Scenario: Invalid requests do not count
- **WHEN** a client IP sends 20 registration requests with a 7-character password within an hour and then a valid one
- **THEN** the invalid requests get 400 and the valid one gets 201

#### Scenario: The limit lifts by itself
- **WHEN** a client IP was limited and its one-hour window has passed
- **THEN** a valid registration from that IP succeeds with 201

#### Scenario: Limit store unavailable
- **WHEN** the limit store cannot be reached and a visitor registers a new address
- **THEN** the response is 201 as if there were no limits

### Requirement: Registration rate-limit message
On a 429 from registration, the web `/register` page SHALL show «Слишком много попыток регистрации. Попробуйте снова через N.», where N is the wait from `retry_after_seconds` rounded up and written as «N сек», «N мин» or «N ч»; without that field the wait MUST be shown as «1 ч». The form MUST stay filled in and usable, MUST NOT show the «Проверьте почту» message, and the message MUST NOT say whether the address is registered.

#### Scenario: Registration is throttled
- **WHEN** a visitor submits the registration form and the API answers 429 with `retry_after_seconds` 600
- **THEN** the form shows «Слишком много попыток регистрации. Попробуйте снова через 10 мин.»

#### Scenario: Throttled without a wait value
- **WHEN** a visitor submits the registration form and the API answers 429 without `retry_after_seconds`
- **THEN** the form shows «Слишком много попыток регистрации. Попробуйте снова через 1 ч.»
