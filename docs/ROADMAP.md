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

## Post-MVP

- [ ] Production launch — architecture, CI/CD, and all code-level prerequisites are done (see [docs/DEPLOYMENT.md](DEPLOYMENT.md)); blocked only on operational provisioning (VPS, domain, GitHub secrets), not on further development
- [ ] RGB/RNB (Russian State/National Library) as an additional catalog data source — investigated, currently blocked: the known Z39.50 endpoint is on a private IP, and the public catalog has no documented API; revisit if RGB responds to direct outreach (see [docs/ai/features/catalog-data-import/business-analysis.md](ai/features/catalog-data-import/business-analysis.md))
- [ ] AI-generated book descriptions as a last-resort fallback when harvest sources have none — discussed, not yet scoped into a feature contract

## Technical Debt

- [ ] TypeScript project references
- [ ] WSL2 backend hot reload
- [ ] Makefile seed
- [ ] README version mismatch