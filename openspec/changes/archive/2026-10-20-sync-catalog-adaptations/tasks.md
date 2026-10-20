# Tasks

## 1. Backend: catalog sync job (screenworks)

- [x] 1.1 Repository: add `ListCatalogSyncBookIDs(ctx)` (published, `external_source = 'wikidata'`, `external_id IS NOT NULL`, ordered by `created_at, id`) to the screenworks repository interface, the GORM implementation and the test fakes. Verify with a repository/service test that pending, rejected and QID-less books are excluded.
- [x] 1.2 Add `screenworks/service/catalog_sync.go`: `TaskTypeCatalogSync = "screenworks:sync_catalog"`, payload `{trigger}`, `NewCatalogSyncTask(trigger)`, `CatalogSyncOptions(queue)` (MaxRetry 3, Timeout 30m, Retention 24h), `CatalogSyncResult` (fields per design Decision 3/API), `CatalogSyncJob.HandleTask` (chunks of 50 via `syncBooks`, progress written after every chunk, errors capped at 50, attempt fails after 3 consecutive failed Wikidata chunks). Verify with unit tests using a fake source/repo/result writer: counters by outcome, progress writes per chunk, error cap with uncapped `failed`, one failed chunk does not abort, three consecutive failures return an error, hidden links are not counted, a progress-write failure does not fail the job.
- [x] 1.3 Add `CatalogSyncInFlight(inspector, queue)` (pending/active/retry, paged, `ErrQueueNotFound` = none) and the tick (`screenworks:sync_catalog_tick`, queue `harvest_schedule`, MaxRetry 0, Timeout 1m, Unique 1h) that skips when a catalog sync is in flight and otherwise enqueues a `scheduled` job on the passed queue. Verify with fake inspector/enqueuer tests (skip on manual in flight, enqueue when idle, harvest tasks ignored, enqueue failure returned).
- [x] 1.4 Run `go build ./...`, `go vet ./...` and `go test ./internal/modules/screenworks/...` from `backend/`.

## 2. Backend: trigger endpoint and jobs list (harvest transport, router)

- [x] 2.1 Add `Handler.TriggerCatalogAdaptationSync` in `harvest/transport/http`: 201 `{task_id, queue, state}`, 409 «adaptation sync already running», 502 on inspector or enqueue failure; swagger annotations (admin role + bearer). Register `books.POST("/sync-adaptations/catalog", ..., requireBearer)` in `internal/transport/http/router.go` before `/:id`. Verify with handler tests (fakes) and a router test for 401 without bearer, 403 for a moderator, 201 for an admin.
- [x] 2.2 Extend `harvestJobResponse` with `kind` and `adaptation_sync` (explicit null) and switch `toHarvestJobResponse` on the task type. Verify with handler tests: a harvest task has `kind: "harvest"`, `adaptation_sync: null`; a catalog sync task has `kind: "adaptation_sync"`, `result: null`, `author_qids: []`, the decoded report and the right trigger. Confirm the scheduled-harvest in-flight test still ignores catalog sync tasks (add a case).
- [x] 2.3 Regenerate swagger (`backend/docs`) and run `go build ./...`, `go vet ./...`, `go test ./internal/modules/harvest/... ./internal/transport/http/...` from `backend/`.

## 3. Backend: catalog filter

- [x] 3.1 Add `HasAdaptation bool` to `catalog/repository.BookFilter`, apply the `EXISTS (... book_screen_works ... NOT hidden)` clause, parse `has_adaptation=true` only in `ListBooks`, add the `@Param` swagger line. Verify with repository/handler tests: visible link included, only-hidden excluded, no link excluded, `total` correct, `has_adaptation=yes` ignored, combined with `genre_id`.
- [x] 3.2 Run `go build ./...`, `go vet ./...`, `go test ./internal/modules/catalog/...` from `backend/`.

## 4. Backend/devops: worker, config, schedule

- [x] 4.1 Config: `AdaptationSyncScheduleEnabled` (`ADAPTATION_SYNC_SCHEDULE_ENABLED`, default false) and `AdaptationSyncScheduleCron` (`ADAPTATION_SYNC_SCHEDULE_CRON`, default `0 5 * * 0`, trimmed). Verify with `config_test.go` cases for defaults and overrides.
- [x] 4.2 Worker: register `TaskTypeCatalogSync` on the harvest mux; start the `harvest_schedule` server when either schedule is enabled and register only the enabled ticks; `registerSchedules` registers the sync tick only when enabled; log `scheduled adaptation sync disabled` otherwise; update the package doc comment. Verify with `cmd/worker/main_test.go` (`fakeRegistrar`: tick registered only when enabled, invalid cron fails) and `go test ./cmd/worker/...`.
- [x] 4.3 Add both env vars to the worker service in `docker-compose.yml` and `docker-compose.prod.yml` next to `HARVEST_SCHEDULE_*` (`${ADAPTATION_SYNC_SCHEDULE_ENABLED:-false}`, `${ADAPTATION_SYNC_SCHEDULE_CRON:-0 5 * * 0}`) and document them in `docs/DEPLOYMENT.md` (off by default, UTC, invalid cron exits the worker).

## 5. Frontend: shared packages and admin

- [x] 5.1 `packages/types` / `packages/api-client`: `AdminHarvestJob.kind`, `AdminHarvestJob.adaptation_sync`, `AdminAdaptationSyncResult`, `triggerCatalogAdaptationSync()`, `has_adaptation` in the books list params. Verify with the api-client tests (query string contains `has_adaptation=true` only when set; trigger hits the right URL with POST).
- [x] 5.2 Admin `entities/harvest`: mutation hook and pure helpers (summary text, reason label, in-flight detection, «Ещё ошибок: N»). Verify with unit tests for the helpers.
- [x] 5.3 Admin `/import/jobs`: button «Найти экранизации для всего каталога» with the confirm dialog and the three banners; disabled while pending or while a catalog sync is in flight; catalog sync row rendering and expanded error table. Respect FSD (view imports entity/feature only). Verify with `harvest-jobs-view.test.tsx` cases from the spec scenarios (start, 409 banner, cancel sends nothing, disabled when active, completed summary, «Ещё ошибок: 70», harvest rows unchanged).
- [x] 5.4 Run `tsc --noEmit` and `next lint` for `apps/admin` and the touched packages (no native `next build`).

## 6. Frontend: web catalog filter

- [x] 6.1 `widgets/catalog-filters` `FilterPanel`: checkbox «Есть экранизация» (`hasAdaptation`, `onHasAdaptationChange`). `views/catalog` `CatalogView`: URL param `has_adaptation=true`, reset page, pass to desktop and mobile panels and to the API params/filter key, chip «Есть экранизация». Verify with `catalog-view.test.tsx` (checking sets the param and drops `page`, a URL with the param shows the checkbox checked and the chip, removing the chip clears it, the API is called with `has_adaptation: true`).
- [x] 6.2 Run `tsc --noEmit` and `next lint` for `apps/web` (no native `next build`; CI is the typedRoutes gate).

## 7. Docs

- [x] 7.1 `docs/admin-guide/import-jobs.md`: the «Найти экранизации для всего каталога» action (what it covers, confirmation, banners, it waits behind or blocks harvests, retries restart from the start), how a «Поиск экранизаций» row and its errors read, and that the weekly automatic run exists only when switched on in the deployment.
- [x] 7.2 `docs/admin-guide/screen-works.md`: link to the catalog-wide action, restate that hidden links stay hidden and hand-edited works are untouched, and mention the public filter «Есть экранизация» (only visible links count, so hiding the only adaptation removes the book from the filter).
- [x] 7.3 Validate the change with `npx -y @fission-ai/openspec@1.13.2 validate sync-catalog-adaptations --strict` (OPENSPEC_TELEMETRY=0).
