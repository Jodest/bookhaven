# Tasks

Ground rules:
- No destructive DB commands: never `migrate down`, `DROP` by hand, or a reset of the local DB. Apply migrations with migrate **up** only.
- gofmt only the files you changed, never the whole tree.
- No native `next build`; CI is the typedRoutes gate.

## 1. Backend

- [x] 1.1 Check the name of the `kind` CHECK constraint with `\d curated_collections` (read-only). Then add the forward migration `backend/migrations/000027_auto_collections.{up,down}.sql` per design.md Database, and apply it with migrate up. Verify that `\d curated_collections` shows `curated_collections_kind_check` allowing `auto`, the rule columns, `computed_at` and `baseline_at`, and that `\d curated_collection_items` shows `previous_position` and `score`.
- [x] 1.2 Domain (`curated/domain`):
  - `KindAuto` and the metric constants;
  - `Rule` with `Normalize`/`Validate` (defaults: trending period 30, min ratings 3, m 10; null `min_ratings`/`confidence` for non-weighted metrics) and its errors;
  - `ErrCollectionAutomatic` («collection is automatic») and `ErrNotAutomatic` («collection is not automatic»);
  - `SnapshotReserve = 20` and `SnapshotLimit(size)`;
  - `RankAuto(rows, rule, limit)` (Bayesian score, eligibility, tie order);
  - `Movement(hasBaseline, prev, pos)`;
  - the new GORM fields.

  Table-driven tests cover:
  - the spec «Many ratings beat a few perfect ones» (4.45 / 4.25 / 3.61);
  - «Below the minimum rating count» (C still includes B);
  - activity eligibility (score ≥ 1);
  - tie order;
  - snapshot limit (size 1000 → 1000, size 100 → 120);
  - every movement direction and the null baseline.

  Verify with `go test ./internal/modules/curated/domain/...`.
- [x] 1.3 Repository:
  - aggregate queries for weighted / shelf-type / trending signals (scope: published, optional genre, optional `since`; portable SQL per design Decision 4);
  - `AutoCollectionIDs(ctx)`;
  - `Recompute` transaction per design Decision 7: lock, re-read, compute through a callback or the service-supplied ranking, old positions unless reset, replace items, set `computed_at`/`baseline_at`;
  - `previous_position`/`score` in `PublishedPage` and `AdminItems`;
  - a `kind = 'manual'` re-check under the lock in `AddBook`, `AddBooks`, `MoveBook` and `RemoveBook`.

  SQLite tests (extend the test schema with `ratings`, `shelves`, `shelf_items` and `book_genres` if missing) cover:
  - pending books and their ratings excluded;
  - genre filter;
  - period cut-off on `ratings.updated_at` and `shelf_items.added_at`;
  - custom shelves ignored;
  - a recompute keeping previous positions;
  - a reset recompute clearing them;
  - a failed compute leaving the old snapshot (rollback);
  - book writes on an auto collection returning `ErrCollectionAutomatic`.
- [x] 1.4 Service:
  - `Create`/`Update` accept `kind`/`rule` with the validation messages from design.md API; `kind` in PATCH → 400;
  - create auto → `Recompute(reset)`, logging an error and still returning the collection;
  - PATCH of an auto collection with a changed rule/genre/size → `Recompute(reset)`;
  - `Recompute(ctx, id)` for the admin action;
  - `RecomputeAll(ctx)` for the worker (continue on per-collection errors, log them);
  - `AddBook`/`RemoveBook`/`MoveBook`/`PreviewPaste`/`BulkAdd` return `ErrCollectionAutomatic` for auto collections;
  - `GetAdmin`/`GetPublic`/`ListPublic`/`ListAdmin` expose `rule`, `computed_at`, `baseline_at`, `score` and `movement`.

  Fake-repository unit tests cover every scenario of both delta specs that is not covered at the transport level.
- [x] 1.5 Transport:
  - request/response shapes per design.md API;
  - `POST /admin/collections/:id/recompute` in the admin `collections` group with `requireBearer`;
  - 422 mapping for both new errors;
  - Russian swagger annotations, then regenerate `backend/docs` the usual way.

  Handler tests cover:
  - create auto with defaults (201);
  - missing rule;
  - rule on a manual collection;
  - PATCH `kind` (400);
  - recompute 200/404/422;
  - add/paste on an auto collection (422);
  - public `movement` null for manual and `up`/`new` for auto.

  Router tests: moderator → 403 and cookie-only → 401 on the recompute route.
- [x] 1.6 Worker and config:
  - `AUTO_COLLECTIONS_SCHEDULE_ENABLED` (default true: unset or unparsable → true, explicit falsy → false) and `AUTO_COLLECTIONS_SCHEDULE_CRON` (default `0 1 * * *`, trimmed) in `internal/config`, with tests like `TestLoad_HarvestSchedule*`;
  - `curated/service` task `curated:recompute_auto`: constructor, options `Unique(1h)` and `MaxRetry(1)`, queue `curated`, and a handler that calls `RecomputeAll`;
  - in `cmd/worker/main.go`, when enabled: a server (`curatedServerConfig`, Concurrency 1), added to `shutdownServers`, plus the schedule in `registerSchedules` with a wrapped error naming the cron;
  - update the package doc comment;
  - extend `main_test.go` so the schedule is registered when enabled, absent when disabled, and an invalid cron fails.
- [x] 1.7 Verify from `backend/`: `go build ./...`, `go vet ./...`, `go test ./...`.

## 2. Frontend

- [x] 2.1 `packages/api-client`: add the types and fields from design.md Frontend, and `admin.collections.recompute(id)` on the bearer admin fetcher. Verify with `tsc --noEmit` for the package.
- [x] 2.2 `apps/web`:
  - `entities/curated-collection/lib/movement.ts` and `lib/auto-caption.ts`, with Vitest tests for every direction, null/`same` → nothing, and the caption with and without a period and before the first computation;
  - `ui/movement-badge.tsx`;
  - `views/collection-detail` renders the badge and the caption. Extend `collection-detail-view.test.tsx` for «↑3», «↓2», «Новинка», no badge for manual, and the exact caption from the spec scenario.
  - `views/collections` keeps working with `kind: 'auto'` items (one test).
- [x] 2.3 `apps/admin` entities and features:
  - entity labels: kind, metric, short-snapshot warning, movement formatter, with tests;
  - `features/manage-collection`: «Тип» radio on create, the rule fields, defaults (weighted → 3 / 10, trending → period 30), mapping to `{kind, rule}` in `lib/collection-form.ts`, with tests;
  - new `features/recompute-collection` (mutation, error text), with tests.

  Respect FSD: features import only entities and shared.
- [x] 2.4 `apps/admin` widgets and views:
  - new `widgets/collection-snapshot`: computed time / «Ещё не пересчитана», baseline line, «Пересчитать сейчас», read-only snapshot table, «Правилу соответствуют только {shown} из {size} книг»;
  - `views/collection-edit` switches between the snapshot widget and `widgets/collection-books` by kind;
  - `views/collections` gets the «Тип» column and the per-kind warning.

  RTL tests:
  - an auto collection shows no catalog search, «Вставить список» or reorder controls;
  - recompute refreshes the table;
  - the short-snapshot warning appears;
  - the manual reserve warning is unchanged;
  - the list shows «Ручная»/«Авто».
- [x] 2.5 Verify:
  - `tsc --noEmit` and `next lint` in `apps/web` and `apps/admin`, and `tsc --noEmit` in `packages/api-client`;
  - the Vitest suites of both apps.

  No native `next build`; wait for the CI frontend job.

## 3. DevOps and docs

- [x] 3.1 Add `AUTO_COLLECTIONS_SCHEDULE_ENABLED` and `AUTO_COLLECTIONS_SCHEDULE_CRON` to the worker service in `docker-compose.yml` and `docker-compose.prod.yml`, next to the `HARVEST_SCHEDULE_*` vars, and document them in `docs/DEPLOYMENT.md`, noting that the schedule is on by default.
- [x] 3.2 Mark epic #413 feature 3 done in `docs/ROADMAP.md` (metrics, snapshot, movement, recompute endpoint, migration `000027_auto_collections`, OpenSpec change `add-auto-collections`). Swagger is regenerated in 1.5. The orchestrator archives the OpenSpec delta; do not edit `openspec/specs/` directly.
