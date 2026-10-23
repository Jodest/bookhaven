## ADDED Requirements

### Requirement: Refresh token retention
The system SHALL permanently delete stored refresh tokens that were revoked (by rotation, logout, an unconfirmed-email refresh or a password reset) or that expired, no earlier than 7 days after the revocation or expiry, by a background purge that runs at least once a day. A token that has not expired and has not been revoked MUST never be deleted by the purge. The purge MUST NOT change any response: a deleted token is answered by `POST /api/v1/auth/refresh` exactly like a revoked or expired one (401). The purge has no endpoint and needs no role.

#### Scenario: Long-revoked token
- **WHEN** a client refreshes with a refresh token that was rotated more than 7 days ago and has since been purged
- **THEN** the response is 401, the same as for a revoked token that is still stored

#### Scenario: Active session survives the purge
- **WHEN** the purge runs while a user holds a refresh token that is neither expired nor revoked
- **THEN** the token is kept and the next refresh with it succeeds
