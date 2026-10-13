# Proposal

## Why

The `auth/authorization` baseline lists three roles, but the code has four: `user`, `contributor`, `moderator` and `admin`. Admins can assign `contributor` from «Пользователи», and the `users` baseline (GitHub issue [#369](https://github.com/Jodest/bookhaven/issues/369)) refers to "the user roles" in `auth/authorization`. This correction makes the spec match the code. No behavior changes.

## What Changes

- "Roles" lists all four roles and says that `contributor` currently grants nothing beyond `user`: no admin endpoint accepts it, and the admin app treats it like a reader.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `auth/authorization`: "Roles" lists `contributor`.

## Impact

- spec only; no code, API or data change.
