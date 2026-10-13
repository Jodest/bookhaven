# Tasks

## 1. Frontend

- [x] 1.1 In `apps/web/src/middleware.ts`, keep guarding `/profile`, `/shelves` and `/shelves/{id}`, but let `/profile/{id}` and `/shelves/public/{id}` through without cookies. Keep the existing comment style and explain the exemption in a short comment.
- [x] 1.2 In `apps/web/src/middleware.test.ts`, add tests: no cookies on `/profile/{id}` and `/shelves/public/{id}` pass through; no cookies on `/shelves/{id}` still redirect to `/login?redirect=/shelves/{id}`; `/profile` keeps redirecting.
- [x] 1.3 Verify in `apps/web`: `./node_modules/.bin/tsc --noEmit`, `./node_modules/.bin/next lint`, `./node_modules/.bin/vitest run src/middleware.test.ts`. Never run a native `next build`.
