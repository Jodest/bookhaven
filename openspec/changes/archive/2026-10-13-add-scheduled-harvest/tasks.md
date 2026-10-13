# Tasks

Never run destructive DB commands (`migrate down`, `drop`, resetting the DB). Apply new migrations only with `migrate up`. Format only the Go files you changed (never `gofmt -w` the whole tree). Never run a native `next build`.

## 1. Backend: fix harvest concurrency (#191)

- [x] 1.1 In `backend/cmd/worker/main.go`, extract the harvest server config into a function that sets `Queues: {harvest: 1}` and `Concurrency: 1`. Update the package doc comment. Verify with a new `main_test.go` test asserting `Concurrency == 1` and the queue map.

## 2. Backend: author auto-harvest flag

- [x] 2.1 Add migration `000021_authors_auto_harvest` (up/down) as in design.md → Database. Add `AutoHarvest` and `LastAutoHarvestedAt` to the author domain model.
- [x] 2.2 Admin service/repository: optional `AutoHarvest *bool` on `AuthorInput` (update only; nil keeps the value). Return `ErrAutoHarvestRequiresQID` when it is `true` on an author without a Wikidata QID. `UpdateAuthor` must never reset the flag or `last_auto_harvested_at`. Add `ListAutoHarvestCandidates(ctx, limit, now)` (eligibility, 30-day floor, order NULLS FIRST, then name, then id) and `MarkAuthorsAutoHarvested(ctx, ids, at)`. Verify with service tests: omitted flag kept, 422 path, and candidate selection over the pending/rejected/no-QID/recent/never cases, ordering and limit.
- [x] 2.3 Admin transport: `auto_harvest` in the PATCH request; `auto_harvest` and `last_auto_harvested_at` in `authorResponse`; map the new error to 422; update the swagger annotations. Verify with handler tests (flag round-trip, 422, create ignores the flag).

## 3. Backend: scheduled harvest

- [x] 3.1 Harvest service: add `Trigger` and `AuthorQIDs` to `HarvestPayload`, plus variadic `HarvestMeta` on `NewHarvestTask` (existing callers unchanged, options unchanged). Verify with a test that the payload round-trips and the options stay MaxRetry 3, Timeout 30m, Retention 24h, queue `harvest`.
- [x] 3.2 Harvest transport: `TriggerHarvest` sends `{trigger: manual, author_qids}`. Job responses expose `trigger` (default `manual`) and `author_qids` (never null), decoded from the task payload. Verify with handler tests for scheduled, manual and legacy (no-field) payloads.
- [x] 3.3 Add the tick handler (`harvest:schedule_tick`, queue `harvest_schedule`, `MaxRetry(0)`, `Timeout(1m)`, `Unique(1h)`) following design.md Decision 4, with small interfaces for the enqueuer, inspector and author source. Verify with fake-based tests: skip when a scheduled job is pending/active/retry, no-op on zero candidates, enqueue plus stamp with the right QIDs and batch, no stamp on enqueue failure, stamp failure does not error.
- [x] 3.4 Config: `HARVEST_SCHEDULE_ENABLED` (default false), `HARVEST_SCHEDULE_CRON` (default `0 4 * * *`), `HARVEST_SCHEDULE_BATCH` (default 10, clamped 1..50). Verify with config tests for defaults and clamping.
- [x] 3.5 Worker wiring: an asynq Client and Inspector, a schedule server (`harvest_schedule`, Concurrency 1) started only when enabled, and `registerSchedules` taking the config (similarity always, tick only when enabled). Shut everything down in reverse order. Verify by extending `main_test.go` for the enabled and disabled cases, then run `go build ./... && go vet ./... && go test ./...` from `backend/`.

## 4. Frontend: admin

- [x] 4.1 `packages/types` / `packages/api-client`: add the author fields and the update input, and the job `trigger` / `author_qids`.
- [x] 4.2 Author edit form: the auto-harvest checkbox, the disabled state and hint without a QID, the last-run line, and the value sent on PATCH (edit mode only). Verify with `author-form.test.tsx` cases (disabled without QID, the PATCH body includes the flag, both last-run texts).
- [x] 4.3 Harvest jobs view: the «Запуск» column with «Авто» / «Вручную» and «Авторов: N», colSpan updated, a trigger-label helper in `entities/harvest/lib` with a unit test. Update `harvest-jobs-view.test.tsx`. Verify with `tsc --noEmit`, `next lint` and vitest in `apps/admin` (plus `tsc` for the touched packages).

## 5. DevOps and docs

- [x] 5.1 Add the three env vars to `docker-compose.yml` and `docker-compose.prod.yml` for the worker service (default off) and to `docs/DEPLOYMENT.md`, with the single-replica note.
- [x] 5.2 `docs/ROADMAP.md`: mark #191 done and add the epic «Автоматическое наполнение каталога» with feature 1 done (link this change). Update the harvest service package doc comment (scheduled trigger, tick queue).
- [x] 5.3 Run `npx -y @fission-ai/openspec@1.13.2 validate add-scheduled-harvest --strict` (with `OPENSPEC_TELEMETRY=0`) and confirm it passes.
