## ADDED Requirements

### Requirement: Reset token retention
Every password reset token, used or not, SHALL be kept for at least 30 days after its expiry, so that within that window the reset endpoint answers `token_used` or `token_expired` exactly as for a fresh lookup. After the window, a background purge that runs at least once a day MUST delete it permanently; the link is then unknown and answers 400 `invalid_token`, which `/reset-password` shows as «Ссылка недействительна» with «Запросить новую ссылку». A token whose expiry has not passed MUST never be deleted by the purge (invalidating unused links on a new request or a successful reset is unchanged).

#### Scenario: Used reset link inside the retention window
- **WHEN** a user submits a valid new password with a reset link 3 days after using it
- **THEN** the response is 400 `token_used`

#### Scenario: Purged reset link
- **WHEN** a user submits a valid new password with a reset link that expired more than 30 days ago and has since been purged
- **THEN** the response is 400 `invalid_token` and the password is unchanged

#### Scenario: Live link survives the purge
- **WHEN** the purge runs 10 minutes after a user requested a reset link
- **THEN** the link still resets the password
