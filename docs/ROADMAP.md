# BookHaven Roadmap

## MVP status: feature-complete (2026-09-30)

Every capability planned for the MVP is implemented, merged to `main`, and covered by its own feature contract under `docs/features/`. Production deployment (see [docs/DEPLOYMENT.md](DEPLOYMENT.md)) is intentionally tracked as a separate, post-MVP milestone: the architecture, CI/CD pipeline, and every code-level prerequisite are already done and smoke-tested, but going live also needs operational steps outside the codebase (provisioning a VPS, buying a domain, populating GitHub secrets) — see the "Post-MVP" section below.

## Completed

- [x] Frontend authentication — silent token refresh fix (see [docs/features/silent-token-refresh.md](features/silent-token-refresh.md))
- [x] Middleware session persistence — stop hard-redirecting protected routes when only the short-lived access token has expired (see [docs/features/middleware-session-persistence.md](features/middleware-session-persistence.md))
- [x] Backend authentication — refresh token hash lookup fix (see [docs/features/refresh-token-hash-fix.md](features/refresh-token-hash-fix.md))
- [x] Catalog
- [x] Shelves
- [x] Ratings and Reviews
- [x] Search and Filters (see [docs/features/search-and-filters.md](features/search-and-filters.md))
- [x] Lists — viewing verified (see [docs/features/lists.md](features/lists.md))
- [x] User Profiles (see [docs/features/user-profiles.md](features/user-profiles.md))
- [x] Admin (see [docs/features/admin.md](features/admin.md))
- [x] Reading Challenge (see [docs/features/reading-challenge.md](features/reading-challenge.md))
- [x] Frontend FSD Migration (see [docs/features/frontend-fsd-migration.md](features/frontend-fsd-migration.md))
- [x] Web Visual Redesign (see [docs/features/web-visual-redesign.md](features/web-visual-redesign.md))
- [x] Catalog data import — async Wikidata/Open Library harvest pipeline, admin trigger UI, progress bar, author name search, full per-job report, Russian-only content enforcement, Open Library work-description enrichment (see [docs/features/async-harvest-queue.md](features/async-harvest-queue.md), [admin-harvest-trigger.md](features/admin-harvest-trigger.md), [harvest-progress.md](features/harvest-progress.md), [harvest-author-search.md](features/harvest-author-search.md), [harvest-job-report.md](features/harvest-job-report.md), [russian-only-content.md](features/russian-only-content.md), [openlibrary-works-description.md](features/openlibrary-works-description.md), [catalog-manual-import.md](features/catalog-manual-import.md))
- [x] Self-hosted fonts — removed the CI build's dependency on live Google Fonts access (see [docs/features/self-hosted-fonts.md](features/self-hosted-fonts.md))
- [x] Notifications for review-like activity — in-app bell + unread badge telling a review's author when someone else likes it (see [docs/features/review-like-notifications.md](features/review-like-notifications.md))
- [x] Admin book list filters — genre, author, and manual/harvested source filters on the admin books list, combinable with the existing search/visibility filters (see [docs/features/admin-book-list-filters.md](features/admin-book-list-filters.md))
- [x] Duplicate author merge — admin action to consolidate a duplicate author record into a canonical one, reassigning its books and removing the duplicate (merge-only scope; no automated duplicate-detection UI, see [docs/features/duplicate-author-merge.md](features/duplicate-author-merge.md))
- [x] Public shelf/reading-status visibility on user profiles — a visitor can see another user's shelves (default and custom) with visible-book-only counts on the public profile and browse a shelf's books read-only, resolving OQ-003 (no privacy toggle) (see [docs/features/public-shelf-visibility.md](features/public-shelf-visibility.md), #95)
- [x] Mailer infrastructure — SMTP sender, async `email:send` queue in the worker, Mailpit for local dev (see [docs/features/mailer-infrastructure.md](features/mailer-infrastructure.md), #157)
- [x] Email verification at registration — verification email with one-time link, verify/resend endpoints with rate limiting (see [docs/features/email-verification-backend.md](features/email-verification-backend.md), #158), `/verify-email` page, check-email panel and site-wide unverified banner (see [docs/features/email-verification-web.md](features/email-verification-web.md), #159); merged to `main` together
- [x] Login only after email verification — unverified users cannot authenticate and use the site as guests; public resend-verification with per-IP/per-email limits (see [docs/features/email-verification-login-gate.md](features/email-verification-login-gate.md), #160, epic #176)
- [x] Similar books, stage 1 (content-based) — "Похожие книги" block on the book page ranked by shared authors/genres, plus the reverse indexes #198 (see [docs/features/similar-books.md](features/similar-books.md), #213, epic #218; merged in #221/#222)
- [x] Password reset by email — forgot/reset endpoints with enumeration-safe responses and per-IP/per-email limits, one-time 1h links, reset also verifies the email (see [docs/features/password-reset.md](features/password-reset.md), #161, epic #181)
- [x] Similar books, stage 2 (readers also enjoyed) — co-reading signal with a 3-reader privacy threshold blended into stage 1, precomputed nightly into `book_similarities` by the worker scheduler (see [docs/features/similar-books-stage2.md](features/similar-books-stage2.md), #217, epic #223; merged in #226/#227)
- [x] Home page content — book of the day, literary calendar with 7-day lookahead and book anniversaries, "Что почитать?" random pick, new/top-rated rows and fresh reviews for everyone; continue reading, challenge card, "Вам может понравиться" and a want-to-read pick for logged-in users (see [docs/features/home-page-content.md](features/home-page-content.md), #229, epic #230; merged in #235/#236 and #237/#238)

## Post-MVP

- [ ] Production launch — architecture, CI/CD, and all code-level prerequisites are done (see [docs/DEPLOYMENT.md](DEPLOYMENT.md)); blocked only on operational provisioning (VPS, domain, GitHub secrets), not on further development
- [ ] RGB/RNB (Russian State/National Library) as an additional catalog data source — investigated, currently blocked: the known Z39.50 endpoint is on a private IP, and the public catalog has no documented API; revisit if RGB responds to direct outreach (see [docs/ai/features/catalog-data-import/business-analysis.md](ai/features/catalog-data-import/business-analysis.md))
- [ ] AI-generated book descriptions as a last-resort fallback when harvest sources have none — discussed, not yet scoped into a feature contract

## Up Next (queued order)

Nothing queued — pick the next item from Proposed via `/backlog-next`. Email notifications (#121, in Proposed) can build on the mailer from #157.

## Proposed (awaiting prioritization)

- [ ] Custom user-created shelves — user-book-shelves business analysis (OQ-001) deferred custom shelves to a future increment needing a schema migration and an exclusivity decision, still unresolved [business-analysis] (proposed 2026-09-30, #94)
- [ ] Follow/friends system with social activity feed — Goodreads' core retention loop of following readers and seeing their activity has no equivalent in BookHaven today [competitive] (proposed 2026-09-30, #96)
- [ ] Personal reading statistics dashboard — The StoryGraph's signature pace/genre/mood analytics have no equivalent beyond BookHaven's single annual reading-challenge count [competitive] (proposed 2026-09-30, #97)
- [ ] Half-star (finer-grained) ratings — Goodreads and The StoryGraph both support half-star ratings; BookHaven's schema restricts ratings to whole integers 1-5 [competitive] (proposed 2026-09-30, #98)
- [ ] Report/flag inappropriate reviews — reviews already have an admin moderation queue, but no user-facing way to flag content for moderator attention, unlike Goodreads/LiveLib [competitive] (proposed 2026-09-30, #99)
- [ ] Personal data export (shelves, ratings, reviews) — Goodreads and The StoryGraph both offer a built-in library export; BookHaven has no data-portability path for a user's own reading data [competitive] (proposed 2026-09-30, #101)
- [ ] Book series / multi-volume grouping in the catalog — the books schema has no series or edition concept at all, unlike Goodreads/LiveLib which group editions under a work and show series membership/order [code] (proposed 2026-09-30, #102)
- [ ] Audit trail for admin actions — admins can hide/edit books and hide/restore reviews, but no log records who performed which administrative action and when [code] (proposed 2026-09-30, #103)
- [ ] Quick shelf-status actions on catalog/search book cards — user-book-shelves business analysis (OQ-004) deferred shelf buttons on listing cards pending a batch per-book shelf-status lookup, still unresolved [business-analysis] (proposed 2026-10-02, #111)
- [ ] Personalized book recommendations based on shelf/rating history — BookHaven collects the shelves/ratings data Goodreads-style recommendation engines use, but has no recommendation surface built on it [competitive] (proposed 2026-10-02, #112)
- [ ] Save and share favorite quotes from books — Goodreads' Quotes feature has no BookHaven equivalent; reviews are the only way to attach text to a book today [competitive] (proposed 2026-10-02, #113)
- [ ] Bulk publish action for pending-review harvested books — the harvest-review-queue contract explicitly deferred a bulk "publish all" action, so admins publish harvested books one row at a time [code] (proposed 2026-10-02, #114)
- [ ] Book clubs / group reading discussions — Goodreads' topic/book-centered group discussions have no BookHaven equivalent, distinct from the already-proposed 1:1 follow/friends feed (#96) [competitive] (proposed 2026-10-02, #115)
- [ ] Reading progress tracking (page/percent updates) for in-progress books — shelves only capture coarse reading status; Goodreads/The StoryGraph let readers log incremental progress on a currently-reading book [competitive] (proposed 2026-10-02, #118)
- [ ] Comments/replies on reviews — reviews only support likes today; Goodreads/LiveLib let readers reply in a comment thread under a review [competitive] (proposed 2026-10-02, #119)
- [ ] Import reading library from Goodreads/StoryGraph CSV — StoryGraph/BookWyrm/Fable all offer Goodreads CSV import for onboarding; BookHaven has no equivalent path for data entering the system (distinct from the proposed own-data export, #101) [competitive] (proposed 2026-10-02, #120)
- [ ] Email notifications for key account and social activity — notifications are in-app only today; Goodreads routinely emails users about friend activity and account events [competitive] (proposed 2026-10-02, #121)
- [ ] User-submitted content warnings on books — The StoryGraph's author-approved and user-submitted content warnings (graphic/moderate/minor severity) have no equivalent in BookHaven's book or review data model [competitive] (proposed 2026-10-02, #122)
- [ ] Reading streaks (consecutive-day habit tracking) — BookHaven only tracks an annual book-count goal; The StoryGraph's day-to-day streak mechanic is a distinct engagement loop with no equivalent [competitive] (proposed 2026-10-02, #123)
- [ ] Barcode/ISBN scanner to add books via device camera — Goodreads and The StoryGraph both let users scan a barcode to add a book instantly; BookHaven's catalog search/add flow has no equivalent [competitive] (proposed 2026-10-02, #124)
- [ ] Track reading format (physical/ebook/audiobook) per book — StoryGraph Plus lets readers log which format they read a book in; BookHaven's shelf/book data model has no format concept [competitive] (proposed 2026-10-02, #125)
- [ ] Private personal notes on books, separate from public reviews — Goodreads lets a user attach a private, non-public note to a book distinct from a review; BookHaven has no non-public annotation field [competitive] (proposed 2026-10-02, #126)
- [ ] Shelf API responses are not camelCase-normalized, causing undefined book counts in the UI — backend shelf endpoints return snake_case while the frontend Shelf type expects camelCase with no normalize step anywhere in the call chain, so shelf.booksCount renders as undefined on the shelves page; flagged as a known latent bug and deferred tech-debt task in docs/ai/features/ratings-reviews/plan.md but never tracked [code] (proposed 2026-10-02, #127)
- [ ] Author detail API response is not camelCase-normalized, breaking photo, dates, and book covers on the author page — api.authors.get fetches the snake_case authorDetailResponse directly as the camelCase Author/Book types with no normalize step (unlike api.books.get), so author.photoUrl/birthDate/deathDate and nested book covers/publishedYear all render as undefined; same root-cause class as #127 on a different endpoint [code] (proposed 2026-10-02, #128)
- [ ] Harvest worker does not actually limit harvest tasks to concurrency 1 — the harvest asynq.Server sets only a queue priority map and leaves Concurrency unset (defaults to NumCPU), so parallel harvests can multiply outbound Wikidata/Open Library request rates despite the documented concurrency 1; flagged in mailer-infrastructure Risks but never filed [code] (proposed 2026-10-03, #191)
- [ ] Passwords longer than 72 bytes return 500 on register and password reset — bcrypt rejects >72-byte input and neither validator has a max length; password-reset contract lists `max=72` as a follow-up [code] (proposed 2026-10-03, #192)
- [ ] Change password for logged-in users — only the emailed forgot/reset flow exists; the password-reset contract left authenticated change-password out of scope [code] (proposed 2026-10-03, #193)
- [ ] Change account email with re-verification — no email change path exists; the email-verification contract left it out of scope while binding tokens to the email to future-proof for it [code] (proposed 2026-10-03, #194)
- [ ] Self-service account deletion — Goodreads and The StoryGraph let users delete their account and data from settings; BookHaven has no deletion path [competitive] (proposed 2026-10-03, #195)
- [ ] No brute-force protection on login — `/auth/login` has no per-IP/per-account rate limit or lockout, unlike resend-verification and forgot-password which already use the in-repo ratelimit package [code] (proposed 2026-10-03, #196)
- [ ] HTTP security headers (CSP, HSTS, X-Frame-Options, nosniff) on web, admin and API — none are set in the Caddyfile, either next.config.js, or the Echo middleware stack [code] (proposed 2026-10-03, #197)
- [ ] Per-page SEO metadata, Open Graph tags, sitemap and robots.txt for public pages — only the root layout has static metadata; book/author/profile pages have no `generateMetadata` and no sitemap/robots exists [code] (proposed 2026-10-03, #199)
- [ ] Custom error and not-found boundaries in web and admin — neither app defines `error.tsx`/`global-error.tsx`/`not-found.tsx`, so failures and unknown slugs show bare Next.js defaults [code] (proposed 2026-10-03, #200)
- [ ] API request logging lacks latency, request ID, user ID and error details — the Echo request logger records only method/uri/status, so slow or failing requests cannot be traced or correlated [code] (proposed 2026-10-03, #201)
- [ ] Frontend tests for core user-facing features — web `rate-book`, `write-review`, `shelve-book`, `add-to-list`, `manage-reading-challenge`, `update-profile` and admin `manage-author`, `manage-genre`, `moderate-review` have no tests [code] (proposed 2026-10-03, #202)
- [ ] Keyboard and screen-reader support for dialogs and popovers — merge-author dialog has no Escape/focus trap/focus return, notification bell popover has no Escape/aria-expanded, and no jsx-a11y/axe checks exist [code] (proposed 2026-10-03, #203)
- [ ] Explicit start/finish reading dates and re-read tracking — the reading challenge uses shelf `added_at` as a completion-date proxy; Goodreads/The StoryGraph let users set dates read and log re-reads [code] (proposed 2026-10-03, #204)
- [ ] Community mood and pace tags on books — The StoryGraph's mood/pace tagging and filtering has no BookHaven equivalent beyond genres (distinct from content warnings, #122) [competitive] (proposed 2026-10-03, #205)
- [ ] Spoiler marking and hiding in reviews — readers can discuss plot twists without spoiling the book for others; Goodreads/LiveLib collapse spoiler-marked reviews, BookHaven reviews have no spoiler concept [competitive] (proposed 2026-10-03, #207)
- [ ] Browse and vote on public community lists — readers can discover themed lists and vote the best books up (Goodreads Listopia); public lists are reachable today only via a direct share URL, with no browse page or votes [competitive] (proposed 2026-10-03, #208)
- [ ] Follow an author and get notified about their new books — readers learn when a favorite author's new book lands in the catalog; Goodreads offers author following, BookHaven has none (distinct from following users, #96) [competitive] (proposed 2026-10-03, #209)
- [ ] Rating distribution and review sorting on the book page — readers see a 1-5 star breakdown and can sort reviews by likes/rating; today the page shows only the average and count, and reviews are newest-first only [code] (proposed 2026-10-03, #210)
- [ ] Themed reading challenges with prompts — readers can pick prompt- or genre-based challenges and tick them off from their shelves (StoryGraph custom challenges, LiveLib community challenges); the current challenge is a single annual book count [competitive] (proposed 2026-10-03, #211)
- [ ] Where to read or buy links to Russian stores on the book page — readers can jump from a book to ЛитРес/Яндекс Книги/MyBook as on LiveLib; the book page and schema have no store links today [competitive] (proposed 2026-10-03, #212)
- [ ] Literary award badges and prize-winner browsing — readers see that a book won or was shortlisted for Большая книга/Ясная Поляна/НОС/Нацбест and can browse each prize's winners (LiveLib, MyBook, FantLab); no award concept exists today [competitive] (proposed 2026-10-03, #214)
- [ ] Sort, filter and search within your own shelves — readers with long shelves can find a book and reorder by title/author/rating/date; shelf books are always listed by date added with no controls [code] (proposed 2026-10-03, #215)
- [ ] "Up Next" queue for the want-to-read shelf — readers can pick and order the few books they will read next from a long want-to-read shelf (The StoryGraph Up Next) [competitive] (proposed 2026-10-03, #216)

## Technical Debt

- [ ] Purge used/expired rows from `email_verification_tokens` (never cleaned up; low volume, see [docs/features/email-verification-backend.md](features/email-verification-backend.md))
- [ ] Purge used/expired rows from `password_reset_tokens` (never cleaned up; same approach as the verification tokens, see [docs/features/password-reset.md](features/password-reset.md))

- [ ] TypeScript project references
- [ ] WSL2 backend hot reload
- [ ] Makefile seed
- [ ] README version mismatch