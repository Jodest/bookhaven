## Context

Issue #508, part 1 of «Книга или экранизация?» (#509 votes and #510 automatic collections follow; do not build them here, but keep the filter definition reusable: "published book with at least one non-hidden link").

What exists:
- `backend/internal/modules/screenworks/service/service.go`: `SyncBooks` (admin, 1..50 ids), `SyncCreatedBooks` (harvest, unbounded, chunked by `SyncCap` = 50) and the private `syncBooks`/`syncChunk`, which already implement every sync rule (add-only, hidden links stay hidden, `manually_edited_at` respected in `repo.SyncBook`). One Wikidata call per chunk of 50 QIDs.
- Harvest jobs: task `harvest:wikidata_query` on queue `harvest`, served by the worker's harvest server with `Concurrency: 1` (`harvestServerConfig`). Options `MaxRetry(3)`, `Timeout(30m)`, `Retention(24h)` (`harvestservice.NewHarvestTask`). The admin jobs list (`harvest/transport/http/handler.go`) lists every task on queue `harvest` and decodes `Result` as `HarvestResult` and the payload's `trigger`/`author_qids` via `harvestservice.PayloadMeta`.
- Scheduled harvest: `harvest:schedule_tick` on queue `harvest_schedule` (own server, Concurrency 1, started only when `HARVEST_SCHEDULE_ENABLED`); the tick inspects queue `harvest` and enqueues an ordinary job. Its in-flight check already filters on `info.Type == TaskTypeHarvestWikidataQuery`, so catalog sync tasks never block it.
- Tables `screen_works`, `book_screen_works (book_id, screen_work_id) PK, hidden` (migration 000029). The PK prefix on `book_id` serves the new `EXISTS` filter; no index needed.

## Decisions

1. **The catalog sync is a task on queue `harvest`** (type `screenworks:sync_catalog`). It shares the single harvest slot, so it never runs in parallel with a harvest (same `WikidataClient`/rate limiter in the worker) and it shows up in the existing jobs list for free. Same options as a harvest: `MaxRetry(3)`, `Timeout(30m)`, `Retention(24h)`. Payload `{"trigger": "manual"|"scheduled"}` — same JSON key as `HarvestPayload.Trigger`, so `harvestservice.PayloadMeta` decodes the trigger unchanged and returns `author_qids: []`.
2. **Code lives in the screenworks module**: `screenworks/service/catalog_sync.go` holds `TaskTypeCatalogSync`, `NewCatalogSyncTask(trigger)`, `CatalogSyncOptions(queue string)`, `CatalogSyncResult`, the job handler (`CatalogSyncJob.HandleTask`), `CatalogSyncInFlight(inspector, queue)` and the tick. The queue name is passed in by callers (`harvestservice.QueueName`), so screenworks does not import harvest; harvest service keeps not importing screenworks (its existing comment). `harvest/transport/http` imports `screenworks/service` for the task, the in-flight helper and the result type (no cycle: screenworks never imports harvest).
3. **Job algorithm**: `repo.ListCatalogSyncBookIDs(ctx)` → `SELECT id FROM books WHERE status = 'published' AND external_source = 'wikidata' AND external_id IS NOT NULL ORDER BY created_at, id`. Write the initial result (`total`, zeros) and then, for each chunk of 50 ids, call the existing private `syncBooks` (same package, so no new export), accumulate counters by outcome, append error entries (reason `wikidata_unavailable` when the item carries `NoteWikidataUnavailable`, else `save_failed`; a `book not found` item — book deleted mid-run — counts as `skipped`), and write the result via `task.ResultWriter()` after every chunk (write errors logged only). Track consecutive chunks whose Wikidata call failed; at 3 return an error so asynq retries (the retry restarts from scratch; already-created links then count as `unchanged`). The final result is written before returning nil. `errors` is capped at 50 entries (`catalogSyncErrorCap`), `failed` is uncapped.
4. **No rotation, no new columns.** The whole eligible set is walked every run: 683 books = 14 SPARQL calls; 5000 books = 100 calls, well inside the 30-minute timeout at the client's throttle. Revisit (per-book `adaptations_synced_at` rotation) only if the catalog grows past ~10k books.
5. **Trigger endpoint** `POST /api/v1/admin/books/sync-adaptations/catalog`, handler `harvesthttp.Handler.TriggerCatalogAdaptationSync` (it already owns the enqueuer and inspector for queue `harvest`). It runs `CatalogSyncInFlight` (pending, active and retry lists of queue `harvest`, paged like `scheduledJobInFlight`; `ErrQueueNotFound` = empty) → 409; inspector error or enqueue error → 502. Registered in `internal/transport/http/router.go` in the `books` group (`adminRole`) with `requireBearer`, before `/:id`.
6. **Scheduled re-sync** (off by default): config `AdaptationSyncScheduleEnabled` (`ADAPTATION_SYNC_SCHEDULE_ENABLED`, default `false`) and `AdaptationSyncScheduleCron` (`ADAPTATION_SYNC_SCHEDULE_CRON`, default `0 5 * * 0`, Sunday 05:00 UTC, an hour after the harvest tick). The scheduler registers a tick task `screenworks:sync_catalog_tick` (empty payload, queue `harvest_schedule`, `MaxRetry(0)`, `Timeout(1m)`, `Unique(1h)`). The tick: `CatalogSyncInFlight` → skip with a log line; otherwise enqueue `NewCatalogSyncTask("scheduled")` with `CatalogSyncOptions(harvestservice.QueueName)`. The `harvest_schedule` server is started when **either** schedule is enabled, and its mux registers only the enabled ticks. When disabled, the worker logs `scheduled adaptation sync disabled`. An invalid cron fails `registerSchedules` (worker exits, existing behavior).
7. **Worker**: register `screenworksservice.TaskTypeCatalogSync` on the existing harvest mux (`mux.HandleFunc`) with a `CatalogSyncJob` built from the existing `screenWorksService` (shares `wikidataClient`). Update the package doc comment.
8. **Jobs list mapping**: `harvestJobResponse` gains `Kind string` (`json:"kind" enums:"harvest,adaptation_sync"`) and `AdaptationSync *screenworksservice.CatalogSyncResult` (`json:"adaptation_sync"`, explicit `null`). `toHarvestJobResponse` switches on `info.Type`: for `TaskTypeCatalogSync` decode `info.Result` into `AdaptationSync` and leave `Result` nil; otherwise today's behavior and `Kind = "harvest"`.
9. **Catalog filter**: `repository.BookFilter.HasAdaptation bool`; the handler sets it only when `c.QueryParam("has_adaptation") == "true"`. Query: `EXISTS (SELECT 1 FROM book_screen_works bsw WHERE bsw.book_id = books.id AND NOT bsw.hidden)`. Same definition #509/#510 will reuse.

## API

- `POST /api/v1/admin/books/sync-adaptations/catalog` (admin + bearer, no body) → 201 `{task_id, queue: "harvest", state: "pending"}`; 409 `{"error": "adaptation sync already running"}`; 502 `{"error": "failed to enqueue adaptation sync job"}` (also for inspector failures); 401/403 from middleware.
- `GET /api/v1/admin/harvest/jobs`, `GET /api/v1/admin/harvest/jobs/:id`: each item gains `kind: "harvest" | "adaptation_sync"` and `adaptation_sync: null | {total, processed, updated, added, unchanged, skipped, failed, errors: [{book_id, title, reason: "wikidata_unavailable" | "save_failed"}]}`.
- `GET /api/v1/books`: new optional `has_adaptation` (`true` filters, anything else ignored).
- Update swagger annotations on all three handlers and regenerate `backend/docs`.

## Database

None. No migration (latest stays `000034`).

## Frontend

- `packages/types` / `packages/api-client`: `AdminHarvestJob` gains `kind: 'harvest' | 'adaptation_sync'` and `adaptation_sync: AdminAdaptationSyncResult | null` (new type). New client method `triggerCatalogAdaptationSync()` → `POST /admin/books/sync-adaptations/catalog`, surfacing the HTTP status so the UI can distinguish 409. `books.list` params gain `has_adaptation?: boolean` (sent as `'true'` only when true).
- Admin `apps/admin/src/entities/harvest`: a mutation hook `useTriggerCatalogAdaptationSync` (invalidates the jobs query on success) and pure helpers in `entities/harvest/lib`, e.g. `isAdaptationSyncJob`, `adaptationSyncSummary(job)`, `adaptationSyncReasonLabel(reason)`, `hasAdaptationSyncInFlight(jobs)`. If the trigger action has UI of its own (confirm dialog + banner), place it as `features/sync-catalog-adaptations` and compose it in the view; the view imports entity/feature only (FSD).
- `apps/admin/src/views/harvest-jobs/ui/harvest-jobs-view.tsx`: header button + confirm dialog + banner (texts in the spec); for `kind === 'adaptation_sync'` rows: «Поиск экранизаций» under the trigger badge, progress from `adaptation_sync.processed/total` while active, the completed summary, «Подробнее» when `adaptation_sync.errors.length > 0`, expanded table (Название, Причина) plus «Ещё ошибок: N». Harvest rows unchanged (they already guard on `job.result`).
- Web `apps/web/src/widgets/catalog-filters/ui/filter-panel.tsx`: props `hasAdaptation: boolean`, `onHasAdaptationChange(v: boolean)`; checkbox «Есть экранизация». `apps/web/src/views/catalog/ui/catalog-view.tsx`: read `has_adaptation === 'true'` from search params (initial state + sync effect), pass to both desktop and mobile `FilterPanel`, `updateURL({ has_adaptation: v ? 'true' : undefined, page: undefined })`, include it in the API params and the filter key, add the chip «Есть экранизация».

## Existing Patterns

- Trigger + 502-on-enqueue: `harvesthttp.Handler.TriggerHarvest`.
- In-flight check over pending/active/retry with paging and `ErrQueueNotFound`: `harvestservice.ScheduleTick.scheduledJobInFlight`.
- Tick on `harvest_schedule`, `registerSchedules` + `fakeRegistrar` test seam: `backend/cmd/worker/main.go` / `main_test.go`; config defaults/clamping tests in `internal/config/config_test.go`.
- Progress via `ResultWriter` after every unit: the harvest handler (`harvest/service/handler.go`).
- Catalog `EXISTS` filters: `catalog/repository/repository.go` (`GenreID`, `AuthorID`).
- Jobs row helpers: `harvestJobTriggerLabel`, `harvestJobAuthorsLabel` in `apps/admin/src/entities/harvest`.

## Risks

- **Wikidata load**: the whole catalog every run. Mitigated by the shared single slot and client throttle, the 3-consecutive-failure stop, the weekly default and the off-by-default schedule.
- **Retry restarts from zero**: idempotent by design, but the retried report counts links created by the failed attempt as `unchanged`. Acceptable; noted in the admin guide.
- **Long job blocks harvests**: a large catalog sync delays queued harvests (and scheduled harvest ticks skip nothing because of it, they simply wait). Acceptable at current size.
- **Admin jobs view assumptions**: any code that assumes every job has a `HarvestResult` must guard on `kind`; the existing view already guards on `job.result`.
- **Single worker replica**: like the other schedules; `Unique(1h)` on the tick and the in-flight check are a defense in depth only.
- **Race on the 409 check**: two simultaneous admin clicks could both pass the inspector check; the duplicate job is harmless (idempotent) and runs after the first.
