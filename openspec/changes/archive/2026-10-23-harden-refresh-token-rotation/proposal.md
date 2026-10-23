# Proposal

Issue: https://github.com/Jodest/bookhaven/issues/629

## Why

Refresh-token rotation reads the token and then revokes it with an unconditional update, so two concurrent refreshes with the same token both mint a valid pair, and a stolen token that was already rotated is simply answered 401 with no signal and no revocation of the session it came from.

## What Changes

- Rotation becomes atomic: the old token is revoked only if it is still active, and the successor is created in the same transaction; of two concurrent refreshes with one token exactly one succeeds.
- Every refresh token belongs to a session family (all tokens descended from one login). Existing tokens become one-token families.
- Reuse detection: presenting a token that was revoked **by rotation** more than 30 seconds earlier revokes every active token of that family (only that session, not the user's other sessions) and logs a warning; the response stays a plain 401, indistinguishable from an unknown token.
- Grace window: re-presenting a token rotated within the last 30 seconds is answered 401 with the new code `refresh_token_superseded` and revokes nothing (benign race between tabs or a lost response).
- The web and admin apps retry the refresh once, after about a second, when they get `refresh_token_superseded`, so a tab that lost a refresh race to another tab of the same browser stays signed in.
- Tokens revoked by logout, password reset, an unconfirmed-email refresh or a family revocation never trigger detection. Purged tokens can no longer be detected (stated explicitly).

## Capabilities

### New Capabilities

### Modified Capabilities

- `auth/session`: atomic rotation, session families, reuse detection with a grace window, the `refresh_token_superseded` retry in the apps, and the purge's effect on detection.

## Impact

- backend: auth service rotation/reuse logic, refresh handler error code, forward migration adding a family id and a rotation link to refresh tokens, tests.
- frontend: shared api-client refresh path (both apps) retries once on `refresh_token_superseded`; test in one app.
- devops: none (migration runs with the normal deploy `migrate up`; safe with the old binary still running).
- docs: DEPLOYMENT.md note on the reuse warning log line; swagger comment of the refresh endpoint.
