# Proposal

## Why

The web app guards every page whose path starts with `/profile` or `/shelves`, so a visitor who is not signed in and opens a reader's public profile (`/profile/{id}`) or a public shelf (`/shelves/public/{id}`) is sent to the login page. The API already serves both to anonymous visitors, and public lists (`/lists/public/{id}`) open for everyone. GitHub issue: [#366](https://github.com/Jodest/bookhaven/issues/366).

## What Changes

- `/profile/{id}` and `/shelves/public/{id}` open for everyone, signed in or not.
- `/profile`, `/shelves` and `/shelves/{id}` stay guarded exactly as before.
- No API, data or copy change.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `auth/authorization`: "Web page guards" exempts the public profile and public shelf pages.
- `collections/public-collections`: "Public pages" no longer sends anonymous visitors of a public shelf to login.

## Impact

- frontend: `apps/web/src/middleware.ts` and its test.
- backend, devops, docs: none.
