# Proposal

Issue: #626.

## Why

`POST /auth/register` checks `name` only as at least 2 characters, untrimmed, while the own-profile rules (`users/own-profile`) require a trimmed name of 2–255 characters. A whitespace-only name is accepted, and a name over 255 characters overflows `users.name VARCHAR(255)` and fails with a 500 instead of a 400. Registration should apply the same name rules as the profile.

## What Changes

- Registration trims `name` and requires 2–255 characters (counted as characters, not bytes); otherwise 400 and no account is created. The trimmed name is stored.
- A name over 255 characters never produces a 5xx response.
- The `/register` form trims the name and checks the same range before sending, with the profile form's message «Имя должно содержать от 2 до 255 символов».

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `auth/registration`: the name follows the profile name rules (trimmed, 2–255 characters) on the endpoint and the page.

## Impact

- backend: auth service `Register` (trim + rune-count check, new error mapped to 400 in the handler), tests.
- frontend: apps/web register form schema, test.
- devops: none.
