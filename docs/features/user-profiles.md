# User Profiles

Status: completed

## Goal

Let authenticated users manage their basic profile and let guests or other users view a privacy-safe public profile.

## User Stories

- As an authenticated user, I can view and edit my name, avatar URL, and profile type.
- As a guest or authenticated user, I can open another user's public profile.
- As a user, I can see my public reading activity without exposing private account data.
- As a user, I cannot edit another user's profile or access their email or password data.

## Acceptance Criteria

- [x] The authenticated profile page loads the current user's profile and provides editing for name, avatar URL, and profile type.
- [x] Valid profile changes persist and are reflected in the current session; invalid input is rejected with a clear Russian error.
- [x] A public profile route loads for guests and authenticated users and displays the user's display name, avatar, profile type, join date, and public lists/activity available through existing read APIs.
- [x] Missing or invalid user IDs return a privacy-safe not-found response and do not reveal account existence beyond the documented response.
- [x] Public responses never expose email, password hash, refresh tokens, or other private fields; owner-only editing is enforced server-side.
- [x] The UI has Russian loading, empty, validation, save-error, and unavailable-profile states and works responsively.
- [x] Logout remains available from the authenticated profile page.

## Business Rules

- A profile is identified by the existing user UUID; no username/slug is introduced for this MVP.
- Only the profile owner may update profile fields. Authentication alone must not authorize updates for an arbitrary user ID.
- `name` is required for display and remains 2–255 characters; trim surrounding whitespace.
- `avatar_url` is optional and must be a valid HTTP(S) URL when supplied; it may be cleared.
- `profile_type` must be one of `reader`, `librarian`, `bookseller`, or `other`.
- Email, password, role, verification state, and timestamps other than the public join date are not editable. Email is owner-only data.
- Public profile activity must reuse existing visibility rules: private lists and other owner-only resources are excluded.

## UX

- Keep `/profile` as the authenticated owner page; add a public profile route using the user's ID and link to it from user identity displays where practical.
- Show avatar fallback, display name, profile type, join date, and public activity. Use Russian labels and feedback.
- Disable or indicate saving while a PATCH is in progress; preserve entered values on validation/server failure.
- Unauthenticated access to `/profile` continues to show the existing sign-in prompt; public profiles remain accessible without a session.

## API

- Add `GET /api/v1/users/:id` (optional auth) returning a public profile DTO and only public activity.
- Add `PATCH /api/v1/users/me` (required auth) accepting `name`, `avatar_url`, and `profile_type`; return the updated owner-safe user DTO. Explicitly sending `avatar_url: null` clears the stored avatar; omitting the field leaves it unchanged.
- Reuse the existing Echo + GORM users module, auth middleware, validator, and JSON naming conventions. Update `/auth/me`/auth response DTOs as needed so avatar data and the saved profile are consistent.
- Return `400` for malformed IDs or validation failures, `401` for unauthenticated mutation, and `404` for unavailable profiles. Never return `password_hash` or tokens.

## Database

No schema changes. Use existing `users.name`, `users.avatar_url`, `users.profile_type`, `users.created_at`, and `users.updated_at` columns and the existing user repository update operation.

## Frontend

- Extend `packages/api-client` and shared user types with public-profile and update methods/DTOs.
- Update `apps/web/src/app/profile/page.tsx` from read-only display to owner editing while retaining auth-store synchronization and logout.
- Add the public profile page and activity links without adding mutation controls to public views.

## Existing Patterns

- Follow the users repository/domain model in `backend/internal/modules/users` and the auth handler's user serialization.
- Follow `apps/web/src/app/profile/page.tsx` and `apps/web/src/lib/auth-store.ts` for authentication/loading behavior.
- Follow the lists public-view contract for optional authentication, privacy-safe DTOs, Russian states, and public/private visibility.

## Tests

- Backend service/handler tests for owner read/update, guest public read, validation, allowed profile types, avatar clearing, malformed/missing IDs, unauthorized mutation, and IDOR attempts.
- Verify DTOs exclude email and sensitive/auth fields from public responses and preserve existing auth response compatibility.
- Frontend checks for owner edit/save/error states, auth redirect/prompt, public guest view, avatar fallback, empty activity, unavailable profile, and no owner controls for visitors.

## Documentation

- Keep this contract as the source of truth. Roadmap status updated to completed.

## Risks

- Existing auth DTOs and shared `User` types disagree on nullability and currently omit avatar data in auth serialization; reconcile this without breaking login/session behavior.
- Public activity scope must not bypass list privacy or expose owner-only fields; keep the first implementation limited to existing public read models.
