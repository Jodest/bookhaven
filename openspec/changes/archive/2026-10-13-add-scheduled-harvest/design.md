## Context

Epic «Автоматическое наполнение каталога», feature 1 of 4. Epic-wide hard rules: AI never writes factual fields, and automation never auto-publishes. Everything harvested lands as `pending` in the existing `/moderation/books` queue.

What exists today:
- `backend/cmd/worker/main.go` runs three `asynq.Server`s (harvest, email, similarity) and one `asynq.Scheduler` (similarity recompute, `30 3 * * *` UTC). It assumes a single replica.
- Harvest jobs are `harvest:wikidata_query` tasks on queue `harvest`. The payload is `{sparql_query}` only. `bookdata.BuildAuthorListQuery(qids, limit)` builds the query. `harvestservice.NewHarvestTask` fixes MaxRetry 3, Timeout 30m and Retention 24h.
- Bug #191: the harvest server sets only `Queues: {harvest: 1}`. `Concurrency` is unset, so it defaults to NumCPU and several harvest tasks run in parallel. Each task shares the worker's single `WikidataClient`/`OpenLibraryClient`, so throttling holds per process, but parallel tasks still interleave and multiply the load in practice. The documented guarantee is "one job at a time".
- An author's Wikidata QID is stored as `authors.external_source = 'wikidata'` + `authors.external_id` (migration 000015).

## Decisions

1. **#191 fix**: add `Concurrency: 1` to the harvest server's `asynq.Config`. Extract the harvest server config into a small function (e.g. `harvestServerConfig(logger) asynq.Config`) so `main_test.go` can assert `Concurrency == 1` and `Queues == {harvest: 1}`.
2. **Opt-in per-author flag.** We do not harvest "all published authors with a QID". A new `authors.auto_harvest` flag defaults to `false`. Eligibility is `auto_harvest AND status = 'published' AND external_source = 'wikidata' AND external_id IS NOT NULL`. The flag can be set on any status (an admin can pre-flag a pending author) but needs a QID (422 otherwise). This keeps outbound load and the moderation inflow under explicit admin control. A later feature (e.g. the AI curator) can widen the list by setting the flag. The "queue of authors marked for harvesting" from the epic is the same flag: newly flagged authors have `last_auto_harvested_at IS NULL` and are picked first.
3. **Rotation**: order `last_auto_harvested_at ASC NULLS FIRST, name ASC, id ASC`. Exclude authors with `last_auto_harvested_at > now() - 30 days` (a fixed constant `autoHarvestMinInterval = 30 * 24h`, not configurable). Batch size comes from `HARVEST_SCHEDULE_BATCH` (default 10, clamped to 1..50; non-integer → default). The job uses the default limit of 500 works (`BuildAuthorListQuery(qids, 0)`). Worst case per run: 1 SPARQL query plus up to 500 Open Library lookups, throttled at about 1 req/s.
4. **Tick task, not a static periodic harvest.** `asynq.Scheduler` registers static payloads, so the scheduler enqueues a lightweight `harvest:schedule_tick` task (empty payload, queue `harvest_schedule`, `MaxRetry(0)`, `Timeout(1m)`, `Unique(1h)`). Its handler:
   1. Uses the inspector to list pending, active and retry tasks on queue `harvest`. If any decodes to `trigger == "scheduled"`, it logs and returns nil (skip).
   2. Calls the admin service `ListAutoHarvestCandidates(ctx, batch, now)` to get the QIDs and IDs. If the list is empty, it returns nil.
   3. Builds the query with `BuildAuthorListQuery` and enqueues `NewHarvestTask(query, HarvestMeta{Trigger: "scheduled", AuthorQIDs: qids})` on `harvest`.
   4. Only after a successful enqueue, calls `MarkAuthorsAutoHarvested(ctx, ids, now)`. A stamp failure is logged and returns nil: the next run will be skipped while the job is pending anyway, and re-harvesting is idempotent.
   An enqueue failure returns the error. The tick has no retries, so the next scheduled run tries again.
5. **Tick runs on its own small `asynq.Server`** (queue `harvest_schedule`, `Concurrency: 1`). It does not run on the harvest server, so ticks never appear in `GET /admin/harvest/jobs` and never wait behind a 30-minute harvest. It does not run on the similarity server either, so it never waits behind a recompute. The worker builds an `asynq.Client` and an `asynq.Inspector` (same `RedisURL`) for the tick handler. Behind small interfaces they satisfy `TaskEnqueuer` / an inspector subset, following the harvest transport's pattern.
6. **Config** (`internal/config`): `HarvestScheduleEnabled` (`HARVEST_SCHEDULE_ENABLED`, default `false` in every env, because there is no prod yet and dev should make no surprise outbound traffic), `HarvestScheduleCron` (`HARVEST_SCHEDULE_CRON`, default `0 4 * * *` UTC, 30 min after the similarity recompute) and `HarvestScheduleBatch` (`HARVEST_SCHEDULE_BATCH`, default 10). When disabled, the tick is not registered and the schedule server is not started. The worker logs `scheduled harvest disabled`. An invalid cron makes `Register` fail, and the worker exits fatally (existing behavior for schedules). `registerSchedules` takes the config values so tests cover both branches.
7. **Job metadata**: `HarvestPayload` gains `Trigger string \`json:"trigger,omitempty"\`` and `AuthorQIDs []string \`json:"author_qids,omitempty"\``. The handler ignores both while processing. `NewHarvestTask(sparql string, meta ...HarvestMeta)` keeps today's callers compiling (`cmd/harvest` passes none → manual). `TriggerHarvest` passes `{Trigger: "manual", AuthorQIDs: <normalized QIDs>}`. Task options are unchanged.
8. Manual harvests do **not** stamp `last_auto_harvested_at`. Rotation stays purely about automation, and the code stays simpler.
9. Scheduled runs reuse the existing harvest processing as-is. Idempotency by `(external_source, external_id)` skips any book with that QID, including rejected tombstones and merged duplicates that kept their external id. Author and genre matching are unchanged.

## API

- `GET /api/v1/admin/harvest/jobs`, `GET /api/v1/admin/harvest/jobs/:id` (admin role): every job item gains `trigger: "manual" | "scheduled"` (decoded from `TaskInfo.Payload`; absent or unknown → `"manual"`) and `author_qids: string[]` (never null; `[]` when absent).
- `PATCH /api/v1/admin/authors/{id}` (admin role + bearer): optional `auto_harvest: boolean` (pointer in the request struct; nil keeps the value). `true` on an author without a Wikidata QID → 422 `{"error": "author has no Wikidata QID"}` (new `ErrAutoHarvestRequiresQID`, mapped to 422 next to the other validation errors). `POST` ignores it.
- `authorResponse` (all admin author payloads) gains `auto_harvest: boolean` and `last_auto_harvested_at: string | null` (RFC 3339).
- Update the swagger annotations on the touched handlers.

## Database

Forward-only migration `000021_authors_auto_harvest.{up,down}.sql`:

```sql
ALTER TABLE authors ADD COLUMN auto_harvest BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE authors ADD COLUMN last_auto_harvested_at TIMESTAMPTZ;
CREATE INDEX idx_authors_auto_harvest_rotation
  ON authors (last_auto_harvested_at NULLS FIRST) WHERE auto_harvest;
```

The down migration drops the index and the columns. It is written for completeness only; never run it. The `catalogDomain.Author` struct gains `AutoHarvest bool` and `LastAutoHarvestedAt *time.Time`. The repository gains `ListAutoHarvestCandidates(ctx, limit, notAfter time.Time)` (eligibility from Decision 2, `last_auto_harvested_at IS NULL OR last_auto_harvested_at <= notAfter`, order from Decision 3) and `MarkAuthorsAutoHarvested(ctx, ids, at)` (a single `UPDATE ... WHERE id IN ?`). `UpdateAuthor` must not overwrite `auto_harvest` / `last_auto_harvested_at` when the input leaves them out. Check the GORM `Save`/`Select` usage in the existing update path, because a zero-valued bool could silently reset the flag.

Author merge: the flag is not transferred from the source to the canonical author (noted in Risks).

## Frontend

- `packages/types` / `packages/api-client`: `AdminAuthor` gains `auto_harvest: boolean` and `last_auto_harvested_at: string | null`. The author update input gains optional `auto_harvest?: boolean`. `AdminHarvestJob` gains `trigger: 'manual' | 'scheduled'` and `author_qids: string[]`.
- `apps/admin/src/features/manage-author/ui/author-form.tsx` (edit mode only): a checkbox «Автоматически пополнять книги из Wikidata». It is disabled with the hint «Нужен Wikidata QID» when `external_source !== 'wikidata' || !external_id`. Under it: «Последний автоматический харвест: <date>» (`ru-RU` date) or «Ещё не выполнялся». Send `auto_harvest` in the PATCH body. The create flow (`/import/authors/manual`) does not render the checkbox.
- `apps/admin/src/views/harvest-jobs/ui/harvest-jobs-view.tsx`: a new column «Запуск» with a Badge «Авто» (scheduled) or «Вручную», and a muted «Авторов: N» line when `author_qids.length > 0`. Bump every `colSpan={4}` to 5. A small pure helper (e.g. `harvestJobTriggerLabel`) in `entities/harvest/lib` keeps this testable. FSD stays intact: the view imports from the entity only.

## Existing Patterns

- The scheduler registration seam: `scheduleRegistrar` + `fakeRegistrar` in `backend/cmd/worker/main_test.go`.
- `TaskEnqueuer` / `TaskInspector` interfaces with hand-rolled fakes: `backend/internal/modules/harvest/transport/http/handler.go`.
- Status-like "omitted keeps current" semantics: `status` on `PATCH /admin/authors/{id}` (`adminservice.AuthorInput`, `service.go` around `UpdateAuthor`).
- Separate-server-per-workload rationale: the worker package doc comment.

## Risks

- **Single replica**: like the similarity schedule, two worker replicas would double ticks. `Unique(1h)` on the tick plus the "scheduled job already queued" check are a defense in depth. Document this next to the existing note.
- **Moderation inflow**: a large flagged list could flood `/moderation/books`. The batch cap (≤50), the 30-day floor and the opt-in flag bound it. With the defaults, at most 10 authors' new works arrive per day.
- **Retry stalls**: a scheduled job stuck in `retry` blocks later runs (by design) until it succeeds or is archived (≤3 retries).
- **Flag reset**: a GORM full-struct update in `UpdateAuthor` could silently reset `auto_harvest` to false, or clear `last_auto_harvested_at`. Cover this with a repository/service test.
- **Author merge** does not carry the flag over. An admin must re-flag the canonical author. This is acceptable for v1.
- The jobs list remains Redis-retention-bounded (24 h). After that, an author's last automatic harvest stays visible only on the author edit page.
