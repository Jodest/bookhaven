## MODIFIED Requirements

### Requirement: Effects of a reset
A successful reset SHALL, in one transaction: mark the token used, replace the password, confirm the email if it was not confirmed (the link proves mailbox ownership), invalidate the user's other unused reset links and revoke all of the user's refresh tokens, ending every session. Two concurrent uses of one link MUST change the password once; the other gets `token_used`. After the transaction commits, the failed-login counter of the account's address MUST be cleared, so a login with the new password is not blocked by attempts made before the reset; a failure to clear it MUST NOT fail the reset.

#### Scenario: Reset ends all sessions
- **WHEN** a user resets the password while logged in on another device
- **THEN** that device can no longer refresh its session

#### Scenario: Reset confirms the email
- **WHEN** a user with an unconfirmed email resets the password
- **THEN** the email is confirmed and the user can log in with the new password

#### Scenario: Reset lifts a login throttle
- **WHEN** the user's address has reached the failed-login limit and the user then resets the password
- **THEN** a login with the new password succeeds immediately
