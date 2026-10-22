## ADDED Requirements

### Requirement: Login rate limits
`POST /api/v1/auth/login` SHALL throttle login attempts with two counters over fixed 15-minute windows: at most 60 attempts of any outcome per client IP and at most 10 failed attempts per submitted email address (compared case-insensitively, ignoring surrounding spaces). A failed attempt is any request that passes request validation but does not present the correct password for an existing account with a password (unknown email, account without a password, wrong password). The per-IP counter counts every attempt that reaches it, including correct passwords, so an attacker cannot keep it flat by alternating attempts against a victim with logins to an account of their own. Both limits MUST be checked before the account is looked up and depend only on the request, so a limited response is identical for existing and unknown addresses. When a limit is reached, the response MUST be 429 with code `rate_limited`, a `retry_after_seconds` body field and a `Retry-After` header equal to the time left in the window, and the password MUST NOT be checked. A request with a correct password (a 200, or a 403 `email_not_verified`) MUST clear the failed-attempt counter of its address but MUST still count against the IP. A request that fails with an internal error MUST NOT count against either counter. A 400 validation error MUST NOT count. The limits MUST NOT lock an account permanently: they lift by themselves when the window ends. When the limit store is unavailable, login MUST keep working without limits rather than fail. The endpoint stays public (no bearer token).

#### Scenario: Too many wrong passwords for one address
- **WHEN** someone sends 10 wrong passwords for an address within 15 minutes and then an 11th attempt
- **THEN** the 11th response is 429 `rate_limited` with `retry_after_seconds` and `Retry-After`, and the password is not checked

#### Scenario: Limited response does not reveal the account
- **WHEN** an address with an account and an address without one each reach the per-address limit
- **THEN** both receive the same 429 `rate_limited` response

#### Scenario: Correct password clears the address counter
- **WHEN** a user mistypes the password 9 times and then logs in with the correct password
- **THEN** the login succeeds and the next wrong password for that address starts a fresh count

#### Scenario: Blocked address even with the right password
- **WHEN** an address has reached its limit and its owner then submits the correct password inside the same window
- **THEN** the response is 429 `rate_limited` and no session is created

#### Scenario: Many addresses from one IP
- **WHEN** one client IP sends 60 attempts spread over different addresses within 15 minutes and then another attempt
- **THEN** the response is 429 `rate_limited` regardless of the address

#### Scenario: Correct passwords count against the IP quota
- **WHEN** one client IP alternates wrong passwords for a victim address with correct logins to its own account until it has made 60 attempts within 15 minutes
- **THEN** the next attempt from that IP is 429 `rate_limited`, even though the per-address counters keep being reset

#### Scenario: The block lifts by itself
- **WHEN** an address was limited and its 15-minute window has passed
- **THEN** a login with the correct password succeeds

#### Scenario: Limit store unavailable
- **WHEN** the limit store cannot be reached and a user logs in with the correct password
- **THEN** the login succeeds as if there were no limits

### Requirement: Login rate-limit message
On a 429 from login, the web `/login` and the admin `/login` SHALL show «Слишком много попыток входа. Попробуйте снова через N.», where N is the wait from `retry_after_seconds` rounded up and written as «N сек», «N мин» or «N ч»; without that field the wait MUST be shown as «15 мин». The form MUST stay usable and the message MUST NOT say whether the account exists.

#### Scenario: Web login is throttled
- **WHEN** a visitor submits the web login form and the API answers 429 with `retry_after_seconds` 600
- **THEN** the form shows «Слишком много попыток входа. Попробуйте снова через 10 мин.»

#### Scenario: Admin login is throttled without a wait value
- **WHEN** staff submit the admin login form and the API answers 429 without `retry_after_seconds`
- **THEN** the form shows «Слишком много попыток входа. Попробуйте снова через 15 мин.»
