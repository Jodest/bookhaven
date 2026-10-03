# Email Verification Web

Status: implemented

> **Changed by #160** ([email-verification-login-gate.md](email-verification-login-gate.md)): the site-wide unverified banner and all logged-in branches of `/verify-email` were removed (unverified users cannot hold a session); register no longer logs in, login shows a not-verified panel with resend, and resend takes an email. The sections below describe the #159 state.

GitHub issue: #159 (epic #162). Depends on #158 ([email-verification-backend.md](email-verification-backend.md), merged into `dev`, not yet `main`). Branch `feat/email-verification-web` is cut from `dev`; #158 and #159 are merged to `main` together (otherwise the emailed link leads to a 404).

## Goal

Close the email verification loop in `apps/web`: after registration the user sees a "check your email" screen, the emailed link `/verify-email?token=...` confirms the address and explains every failure state, and logged-in unverified users see a site-wide banner with a rate-limit-aware "send again" button. No restrictions for unverified users (that is #160).

## Scope boundaries

**In scope:** check-email state after register, `/verify-email` page, site-wide banner, shared resend button/logic, small `RegisterForm` change, tests, docs.

**Out of scope:** restricting unverified users (#160), password reset (#161), admin app UI (`apps/admin` only needs to keep compiling - `User.emailVerified` already exists), backend/API changes, api-client changes (everything needed already exists in `packages/api-client/src/index.ts`: `User.emailVerified`, `api.auth.verifyEmail`, `api.auth.resendVerification`, `ApiError.code`/`retryAfterSeconds`, `isApiError`, `VerificationErrorCode`, verify-email in `AUTH_REFRESH_EXCLUDED_PATHS`).

## User Stories

- As a new user, right after registering I see that a confirmation email was sent to my address, can request it again, and can continue into the app without confirming first.
- As a user who clicked the link (logged in or not, any browser), I see clearly whether my email is now confirmed, was already confirmed, or the link is expired/invalid/used - and what to do next.
- As a logged-in unverified user, I see a banner on every page reminding me to confirm and letting me resend the email; I am told when I must wait before resending.
- As a user who just confirmed, the banner disappears without a manual reload.

## Acceptance Criteria

- [x] After successful registration the register page switches in place (no navigation) to a "Проверьте почту" panel showing the entered email, a resend button and a "Продолжить" link to the safe `redirect` param or `/profile` (same rule as today via `isSafeRedirectPath`).
- [x] `/verify-email` is reachable logged in and logged out (no middleware change needed - it is neither in `protectedRoutes` nor `authRoutes`; verify this stays true).
- [x] `/verify-email` with a non-empty `token` posts it exactly once per page load, including under React StrictMode double-mount, and renders the state per the UX table; without a token it does not call the API and shows the "missing token" state.
- [x] After `verified` / `already_verified` the page calls `useAuthStore.getState().fetchUser()` so the header/banner reflect `emailVerified: true`.
- [x] Failure states (`token_expired`, `invalid_token`, `token_used`): logged-in unverified -> resend button; logged-in verified -> "Ваш email уже подтверждён" + link to profile; logged out -> "Войдите, чтобы получить новое письмо" + link `/login`. Generic error -> "Попробовать снова" button re-posts the token.
- [x] Banner is rendered from `app/layout.tsx` directly under the header, visible only when session loaded, user present, `user.emailVerified === false`, not dismissed for this user in this browser session, and pathname is not `/register` or `/verify-email`.
- [x] Resend (banner, check-email panel, verify page all share one component): 202 -> success message, button disabled 60 s; 429 -> "Слишком много запросов. Попробуйте снова через N сек/мин/ч." using `retryAfterSeconds` (fallback 60), button disabled until then; 409 -> `fetchUser()` (banner disappears) and "Email уже подтверждён"; 500 / network / other -> generic error message, button re-enabled.
- [x] Banner dismiss button (session-only, per user id, `sessionStorage`) hides it until the tab session ends; it reappears after re-login as a different user.
- [x] All copy in Russian; visual style uses existing tokens and `@bookhaven/ui` `Button`; layout matches login/register card style.
- [x] Existing `RegisterForm` tests still pass; new unit/component tests (see Tests) pass; `pnpm` typecheck/lint (incl. FSD boundaries) /test clean for web and admin.

## Business Rules

- The verify call is public; it never logs the user in and never touches cookies. The page must not assume the logged-in user owns the token (it may belong to another account) - always re-fetch the user from `/auth/me` rather than optimistically setting `emailVerified: true`.
- Resend acts only on the authenticated user (server-side); the client only offers it when `isAuthenticated && user && !user.emailVerified`.
- Registration's own email does not count against the resend quota, so the first resend from the check-email panel works immediately (backend min interval then 60 s).
- Token is never logged, stored, or sent anywhere except the single `verifyEmail` call. Page sets `referrer: 'no-referrer'` and `robots: noindex` in metadata so the token in the URL does not leak via Referer or indexing.
- Do not validate token format on the client beyond "non-empty"; the backend returns `invalid_token` for malformed tokens.
- Dismissing the banner is cosmetic and session-only; it never affects server state.

## UX

### Check-email panel (register page, after success)

Same card as `RegisterView`. Title "Проверьте почту". Text: "Мы отправили письмо со ссылкой для подтверждения на **{email}**. Ссылка действительна 24 часа." Secondary text: "Не пришло письмо? Проверьте папку «Спам» или отправьте его ещё раз." + resend button ("Отправить письмо ещё раз", `variant="outline"`). Primary `Button asChild` -> `Link` "Продолжить" (safe redirect or `/profile`). Note under it (muted): "Подтвердить email можно позже - ссылка из письма будет работать 24 часа." Reloading `/register` while logged in redirects to `/profile` via middleware - acceptable, the banner takes over there.

### `/verify-email` page

Centered card like login/register. One state at a time:

| State | Title | Body | Actions |
|---|---|---|---|
| `loading` | Подтверждаем email… | - | - (spinner/text, `aria-live="polite"`) |
| `verified` | Email подтверждён | Спасибо! Адрес электронной почты подтверждён. | logged in: "Перейти в профиль" (`/profile`), "К каталогу" (`/books`); logged out: "Войти" (`/login`) |
| `already_verified` | Email уже подтверждён | Этот адрес уже был подтверждён ранее. | same as `verified` |
| `token_expired` | Срок действия ссылки истёк | Ссылка действительна 24 часа. Запросите новое письмо. | resend block (see below) |
| `invalid_token` | Ссылка недействительна | Ссылка повреждена или устарела. Если вы запрашивали письмо повторно, используйте ссылку из последнего письма. | resend block |
| `token_used` | Ссылка уже использована | Эта ссылка уже была использована. | resend block |
| `missing_token` | Ссылка неполная | В ссылке нет кода подтверждения. Откройте ссылку из письма целиком. | resend block |
| `error` | Не удалось подтвердить email | Произошла ошибка. Проверьте подключение и попробуйте снова. | "Попробовать снова" (re-post same token) |

Resend block: while session `isLoading` render nothing; logged in + unverified -> shared resend button; logged in + verified -> "Ваш email уже подтверждён." + "Перейти в профиль"; logged out -> "Войдите, чтобы получить новое письмо." + "Войти" (`/login`; after login the banner offers resend).

### Banner (site-wide)

Full-width strip between `Header` and page content: `border-b bg-secondary text-secondary-foreground`, inner `container mx-auto px-4 sm:px-6 lg:px-8 py-2 text-sm`, flex-wrap. Text: "Подтвердите адрес электронной почты: мы отправили ссылку на **{email}**." + resend button (`size="sm"`, `variant="outline"`) + dismiss icon button (`variant="ghost" size="icon"`, `aria-label="Скрыть"`, "×"). Feedback text next to the button in an `aria-live="polite"` span. Wrapper `role="region" aria-label="Подтверждение email"`. Renders `null` until conditions hold (client-only, so no hydration mismatch).

Cross-tab: user registers in tab A, opens link in tab B; returning to tab A triggers the existing `visibilitychange -> fetchUser()` in `SessionInitializer`, so the banner disappears there too.

### Resend messages (shared)

- sending: button text "Отправляем…", disabled.
- sent: "Письмо отправлено. Проверьте почту." (button disabled 60 s).
- rate_limited: "Слишком много запросов. Попробуйте снова через {N сек | N мин | N ч}." - `formatRetryAfter(s)`: `s < 60` -> `${s} сек`; `s < 3600` -> `${ceil(s/60)} мин`; else `${ceil(s/3600)} ч` (abbreviations avoid Russian plural forms). Button disabled for `s` seconds (re-enabled via timeout, no live countdown).
- already_verified: "Email уже подтверждён." + `fetchUser()`.
- error: "Не удалось отправить письмо. Попробуйте позже."

## API

No changes. Uses existing endpoints via api-client (see [email-verification-backend.md](email-verification-backend.md#api)):
- `api.auth.verifyEmail(token)` -> `{status: 'verified' | 'already_verified'}`; errors `ApiError` 400 with `code` `invalid_token | token_expired | token_used`.
- `api.auth.resendVerification()` -> 202 `{status: 'sent'}`; errors 409 `already_verified`, 429 `rate_limited` + `retryAfterSeconds`, 500 `email_send_failed`. 401 goes through the normal reactive refresh.

## Frontend

All in `apps/web/src`, respecting FSD boundaries in `apps/web/.eslintrc.json` (features cannot import sibling features; `app/` route files may import only `shared` + `views`; `app/layout.tsx` may import widgets).

- `features/resend-verification/` (new)
  - `model/resend-state.ts` - pure: `type ResendState = {kind:'idle'} | {kind:'sending'} | {kind:'sent'} | {kind:'rate_limited'; retryAfterSeconds:number} | {kind:'already_verified'} | {kind:'error'}`; `resendErrorToState(err: unknown): ResendState` (uses `isApiError`, `code`, `status`; 429 without `retryAfterSeconds` -> 60); `formatRetryAfter(seconds: number): string`; `resendMessage(state): string | null`.
  - `model/use-resend-verification.ts` - hook: `{ state, resend, disabled }`; calls `api.auth.resendVerification()`; on `already_verified` calls `useAuthStore.getState().fetchUser()`; manages the disable timeout (cleared on unmount).
  - `ui/resend-verification-button.tsx` - button + live message; props `variant?`, `size?`, `className?`.
  - `index.ts` exports the button, hook, `formatRetryAfter`.
- `features/verify-email/` (new)
  - `model/verify-state.ts` - pure: `type VerifyState = 'loading' | 'verified' | 'already_verified' | 'invalid_token' | 'token_expired' | 'token_used' | 'missing_token' | 'error'`; `verifyResultToState(res)`, `verifyErrorToState(err)` (ApiError 400 with known code -> that code; 400 with unknown/missing code -> `invalid_token`; anything else -> `error`).
  - `model/use-verify-email.ts` - hook `useVerifyEmail(token: string | null): { state, retry }`. StrictMode guard: a `useRef<string | null>` holding the token already submitted; the effect returns early if `ref.current === token`. **Do not** use an effect-cleanup "cancelled" flag that discards the response - under StrictMode it would drop the only in-flight request's result and leave the page stuck on `loading`. On `verified`/`already_verified` call `useAuthStore.getState().fetchUser()`. `retry()` resets to `loading` and re-posts.
  - `index.ts`.
- `features/auth/ui/register-form.tsx` - add optional prop `onRegistered?: (email: string) => void`; when provided, call it after successful `registerUser` instead of `router.push` (existing behaviour unchanged when absent, so existing tests stay valid).
- `views/register/ui/register-view.tsx` - becomes a client component holding `registeredEmail` state; renders `RegisterForm onRegistered={setRegisteredEmail}` or new `views/register/ui/check-email-panel.tsx` (uses `ResendVerificationButton`, `useSearchParams` + `isSafeRedirectPath` for the continue link). Page already wraps the view in `Suspense`.
- `views/verify-email/` (new) - `ui/verify-email-view.tsx` reads `token` via `useSearchParams` (trimmed; empty -> `missing_token`), uses `useVerifyEmail`, `useAuthStore` (user, isAuthenticated, isLoading) and `ResendVerificationButton`; card layout per UX table. `index.ts`.
- `app/verify-email/page.tsx` (new) - `metadata = { title: 'Подтверждение email — BookHaven', robots: { index: false, follow: false }, referrer: 'no-referrer' }`; renders `<Suspense><VerifyEmailView /></Suspense>` (same as `app/register/page.tsx`).
- `widgets/email-verification-banner/` (new) - `ui/email-verification-banner.tsx` (uses `useAuthStore`, `usePathname`, `ResendVerificationButton`); dismissal in `sessionStorage` key `bookhaven:verify-banner-dismissed:<userId>`, read in `useEffect` (guard `try/catch` for storage access); `index.ts`.
- `app/layout.tsx` - render `<EmailVerificationBanner />` between `<Header …/>` and the content `div`.
- `middleware.ts` - no change; add a one-line comment near `protectedRoutes` that `/verify-email` must stay public.

## Existing Patterns

- Client page with search params wrapped in `Suspense`: `app/register/page.tsx`, `views/register/ui/register-view.tsx` (card styling).
- Auth state: zustand `entities/session/model/auth-store.ts` (`user`, `isLoading`, `isAuthenticated`, `fetchUser`, `getState()`); loaded by `SessionInitializer` in `app/providers.tsx`. No react-query for the current user.
- Safe redirect: `shared/lib/safe-redirect.ts` (`isSafeRedirectPath`), see [auth-return-url.md](auth-return-url.md).
- Feature calling api-client directly with `isApiError`: `features/manage-reading-challenge/ui/manage-reading-challenge-goal.tsx`, `features/update-profile`.
- Widget injected from app-root: `widgets/notifications` in `app/layout.tsx`.
- Tests: `features/auth/ui/register-form.test.tsx` (mocks `next/navigation`, `@/entities/session`), `widgets/notifications/ui/notification-bell.test.tsx`, `entities/session/model/auth-store.test.ts` (mocks `@bookhaven/api-client`).

## Tests

Vitest + Testing Library, following the mocking style above (`vi.mock('@bookhaven/api-client')` with a real `ApiError` class re-exported via `vi.importActual`, or construct `ApiError` from the actual module).

- `features/resend-verification/model/resend-state.test.ts`: 429 with `retryAfterSeconds` -> rate_limited with that value; 429 without -> 60; 409 `already_verified`; 500 `email_send_failed`; non-ApiError (TypeError) -> error. `formatRetryAfter`: 1 -> "1 сек", 59 -> "59 сек", 60 -> "1 мин", 61 -> "2 мин", 3599 -> "60 мин", 3600 -> "1 ч", 86400 -> "24 ч".
- `features/verify-email/model/verify-state.test.ts`: each response/code mapping; 400 without code -> `invalid_token`; 500 / network -> `error`.
- `features/verify-email/model/use-verify-email.test.tsx` (or view test): rendered inside `<React.StrictMode>` -> `api.auth.verifyEmail` called once and final state `verified`; `fetchUser` called after success and after `already_verified`; null token -> no call, `missing_token`; `retry()` after error calls again.
- `views/verify-email/ui/verify-email-view.test.tsx`: `token_expired` + logged-in unverified -> resend button; logged out -> "Войти" link; logged-in verified -> no resend button.
- `widgets/email-verification-banner/ui/email-verification-banner.test.tsx`: hidden when loading / logged out / verified / pathname `/verify-email`; shown with email for unverified; resend 202 -> success text; 429 (`retryAfterSeconds: 90`) -> "через 2 мин" and button disabled; 409 -> `fetchUser` called; dismiss -> hidden and `sessionStorage` key set for that user id; another user id -> shown.
- `features/auth/ui/register-form.test.tsx`: add case - with `onRegistered`, it is called with the email and `push` is not called.
- `views/register/ui/register-view.test.tsx` (optional): after submit shows "Проверьте почту" with the email and continue link honouring a safe `redirect`.
- Manual (Mailpit, `docker compose up`): register -> panel; open link logged out and logged in; open it again -> already verified; tamper token -> invalid; resend twice quickly -> 429 message; banner disappears after verifying in another tab and returning.

## Documentation

- `docs/ROADMAP.md`: #158 line -> implemented (in `dev`, waiting for #159 to merge to `main` together); #159 line -> in progress with link to this contract.
- This contract: set `Status: implemented` and add "Implementation Notes (as built)" when done.
- `docs/features/email-verification-backend.md`: no change required (optionally link this contract in the header).

## Risks

- **StrictMode / double submit**: a naive effect would POST twice; the second call returns `already_verified`, causing a flicker from "подтверждён" to "уже подтверждён". Ref guard mitigates; the "cancelled" cleanup anti-pattern would leave the page stuck on loading - covered by the StrictMode test.
- **Token belongs to another account** than the logged-in one: handled by re-fetching `/auth/me` instead of optimistic update; banner stays for the logged-in (still unverified) account, which is correct.
- **Race with `SessionInitializer`**: `/auth/me` may complete before the verify transaction and return stale `emailVerified: false`; the explicit `fetchUser()` after verify fixes it.
- **Register page reload** loses the check-email panel (middleware redirects authenticated users from `/register` to `/profile`); the banner covers this - acceptable.
- **Banner noise**: shown on every page until verified; mitigated by session-only dismiss. After #160 the banner becomes more important - keep it non-dismissible-forever by design.
- **Token leakage via URL** (history, Referer): mitigated by `referrer: 'no-referrer'` + noindex; history exposure accepted (24h, one-time).
- **FSD lint**: check-email panel must live in `views/register` (not `features/auth`) because it composes two features.
- **Merge order**: must reach `main` together with #158; do not merge `dev` -> `main` partially.

## Implementation Notes (as built)

- `features/resend-verification`: `resendErrorToState` treats `status === 429 || code === 'rate_limited'` as rate-limited (non-positive/missing `retryAfterSeconds` -> 60, fractional values rounded up) and `409 || code === 'already_verified'` as already verified; everything else -> `error`. `formatRetryAfter` clamps to at least 1 s. The hook keeps a separate `coolingDown` flag driven by a single timeout (60 s after `sent`, `retryAfterSeconds` after 429), cleared on unmount; `error` leaves the button enabled. `ResendVerificationButton` defaults to `variant="outline"`; its message span is `aria-live="polite"`, error/rate-limit text uses `text-destructive`.
- `features/verify-email`: `verifyResultToState` maps an unexpected success body to `error`. `useVerifyEmail` initialises to `missing_token` when the token is null, so a missing token never flashes `loading`. Ref guard as specified; no cleanup-cancel flag.
- `views/verify-email`: `SuccessActions` (verified/already_verified) and the resend block both render nothing while the session is loading. The title/body container is `aria-live="polite"`.
- `widgets/email-verification-banner`: dismissal state is stored as `{userId, dismissed}` and read from `sessionStorage` in an effect keyed on the user id, so the banner renders nothing until that read completes for the current user (no hydration mismatch, no flash for a dismissed user, re-evaluated on user switch). Storage errors are swallowed (read -> not dismissed; write -> hidden for the current mount only).
- `views/register`: `RegisterView` is now a client component; `check-email-panel.tsx` holds the "Проверьте почту" panel. `RegisterForm` gained `onRegistered?(email)`; without it behaviour is unchanged.
- `middleware.ts`: comment only; `/verify-email` remains in neither `protectedRoutes` nor `authRoutes`.
- Tests: `resend-state.test.ts`, `verify-state.test.ts`, `use-verify-email.test.tsx` (StrictMode), `verify-email-view.test.tsx`, `email-verification-banner.test.tsx`, `register-view.test.tsx`, plus the `onRegistered` case in `register-form.test.tsx`.
- Manual check (local docker stack + Mailpit, headless Chrome over CDP): register -> panel with email and safe continue link (`?redirect=/books`), banner hidden on `/register`; first resend from the panel -> "Письмо отправлено", button disabled; banner on `/books` -> resend -> 429 "через 57 сек", button disabled; tampered token -> "Ссылка недействительна" + resend; no token -> "Ссылка неполная"; real link logged in (dev StrictMode) -> "Email подтверждён" (single POST, no "уже" flicker) -> banner gone after client navigation without reload; reopen link -> "Email уже подтверждён"; logged out -> "уже подтверждён" + "Войти", invalid link logged out -> "Войдите, чтобы получить новое письмо.". An older emailed token superseded by a resend returns `invalid_token`, which matches the copy for that state. Note: the `bookhaven-web` dev container did not pick up the new route until it was restarted (Windows bind mount does not deliver file-watch events).
