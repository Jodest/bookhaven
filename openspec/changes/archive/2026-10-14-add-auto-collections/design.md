## Context

Epic #413 feature 3. Features 1-2 built manual collections in `backend/internal/modules/curated` (tables `curated_collections` and `curated_collection_items`, places computed at read time by `domain.Rank`), public `/top` and `/top/{slug}` in `apps/web`, and the admin section «Подборки» in `apps/admin`, including bulk paste. This change adds kind `auto`: a rule-driven ranking that the worker stores as a snapshot in the same items table. Rating data:
- `ratings` (one row per user and book, `value` 1-5, `created_at`, `updated_at`) is the single source of ratings. Reviews write their rating there too.
- `books.rating` / `books.ratings_count` are its rounded aggregates.

Reading status:
- `shelves.type` (`read`, `want_to_read`, `reading` are the default shelves; a book is on at most one of them per user).
- `shelf_items.added_at`.

The worker already runs an `asynq.Scheduler` (similarity daily, scheduled harvest gated by `HARVEST_SCHEDULE_ENABLED`, AI sweep).

## Decisions

1. **Metrics v1: `weighted_rating`, `most_read`, `most_wanted`, `trending`.**
   - `weighted_rating` is the agreed «Топ-100».
   - `most_read` and `most_wanted` reuse the existing reading-status signal with no new data.
   - `trending` gives a «сейчас читают» collection.
   - «Most reviewed» is left out. Every review carries a rating, so it is mostly a noisier `ratings_count`, and review volume is low. It can be added later by widening the metric CHECK.
2. **Bayesian score** `(v·R + m·C)/(v + m)`, computed in Go from per-book aggregates, not from the rounded `books.rating`.
   - R and v come from the counted rows of `ratings` (published books, optional genre, optional period on `ratings.updated_at`). C is `SUM(value)/COUNT(*)` over the same scope, including books below `min_ratings`.
   - Defaults are m = 10 and min ratings = 3:
     - m = 10 means a book needs about 10 ratings before its own average outweighs the site mean, which is the right scale for a young catalog without a cold-start special case.
     - min 3 drops single-vote noise.
   - Both values are stored in the rule and tunable from 1 to 10000.
   - There is no cold-start logic: fewer qualifying books simply give fewer places, and the admin sees the «Правилу соответствуют только …» warning.
   - Anti-manipulation relies on the existing invariant that only verified accounts can authenticate, and so rate (`auth/email-verification`). No new code.
3. **Period semantics.** For ratings, `ratings.updated_at >= now - period` counts: the current value dates from its last change. For shelves, `shelf_items.added_at >= now - period` counts, using the current shelf state, so a book moved off «Прочитано» no longer counts. `trending` is the count of ratings set or changed in the period plus reading-status shelf additions (`read`, `reading`, `want_to_read`) in the period. Its period defaults to 30. Activity metrics require a score of at least 1.
4. **Ranking in Go, aggregation in SQL.**
   - The repository returns one row per candidate book: `book_id`, `title`, `books.ratings_count`, plus `rating_sum` / `rating_count` or `signal_count`.
   - The pure function `domain.RankAuto(rows, rule, limit)` scores, filters, sorts (score DESC, `ratings_count` DESC, title ASC, id ASC) and truncates.
   - This keeps the math unit-testable and the SQL portable for the SQLite repository tests (plain `GROUP BY`, no `::` casts or window functions).
   - Volume is bounded by the number of rated or shelved published books, which is fine at the current scale (see Risks).
5. **Snapshot in `curated_collection_items`.**
   - `position` is the snapshot rank 1..k, with `k = min(size + 20, 1000)` (`domain.SnapshotReserve = 20`).
   - New nullable columns: `score` (DOUBLE PRECISION) and `previous_position`.
   - Only books published at compute time are written. Public places reuse `domain.Rank` and `PublishedPage` unchanged. An unpublished book is skipped at read time and the reserve fills place N, consistent with manual collections. Movement still compares stored positions, so readers see ranks that are stable during the day.
6. **Baseline.**
   - `curated_collections.computed_at` holds the time of the current snapshot. `baseline_at` holds the time of the snapshot that `previous_position` refers to, or NULL.
   - A normal recompute (scheduled or «Пересчитать сейчас») reads the current items as the previous positions and sets `baseline_at = old computed_at`.
   - Create, or a change of `rule` / `genre_id` / `size`, recomputes with reset: `previous_position` NULL and `baseline_at` NULL.
   - Movement (`domain.Movement`):
     - `baseline_at` NULL → null;
     - `previous_position` NULL → `new`;
     - otherwise compare positions → `up`/`down` with `by = |prev - pos|`, or `same`.
   - Pressing «Пересчитать сейчас» twice makes the baseline the snapshot from minutes ago. This is accepted and the UI shows the baseline time.
7. **Recompute transaction.**
   - `Service.Recompute(ctx, id, reset bool)` runs one transaction:
     1. `lock` the collection row (`SELECT … FOR UPDATE`, existing helper);
     2. re-read the collection, returning `ErrNotAutomatic` if it is manual;
     3. run the aggregate query for its saved rule;
     4. read the old positions unless `reset`;
     5. `DELETE` the items and insert the new ones;
     6. update `computed_at`, `baseline_at` and `updated_at`.
   - Because the rule is read under the lock, a concurrent PATCH (whose `UPDATE` waits for the row lock) can never be overwritten by a snapshot of the old rule. PATCH then runs its own reset recompute after commit.
   - If a recompute fails, the transaction rolls back and the old snapshot stays.
8. **«Пересчитать сейчас» is synchronous in the API** (`POST /admin/collections/{id}/recompute`).
   - The work is one aggregate query plus at most 1000 inserts, so a job/polling UI is not worth it. The admin gets the new snapshot in the response.
   - Create and PATCH call the same method. On create, a compute error is logged and the 201 is still returned with `computed_at` null.
9. **Worker schedule.**
   - New task `curated:recompute_auto` on its own small server: queue `curated`, Concurrency 1, `asynq.Unique(1h)`, `MaxRetry(1)`.
   - It is registered on `AUTO_COLLECTIONS_SCHEDULE_CRON` (UTC, default `0 1 * * *`, 04:00 MSK, before the 03:30 UTC similarity job).
   - The handler lists the ids of every `auto` collection (draft and published) and calls `Recompute(id, false)` for each, logging and continuing on per-collection errors. It returns an error only when listing fails.
   - `AUTO_COLLECTIONS_SCHEDULE_ENABLED` is **on by default**, unlike the harvest flag. The job makes no external calls and costs little, and without it automatic collections never refresh. The flag exists so dev/tests can silence it. Unset or unparsable means true; an explicit falsy value disables it.
   - When disabled, neither the schedule nor the server is set up, the same as the harvest tick.
   - The single-worker-replica assumption is the same as for the other schedules.
10. **Kind is fixed at creation.** Converting between kinds would need rules for what happens to the admin order and the snapshot. That is not needed now.
11. **Manual-only book endpoints** check the kind in the service (`ErrCollectionAutomatic` → 422 «collection is automatic») after the role/bearer middleware. The repository write paths (`AddBook`, `AddBooks`, `MoveBook`, `RemoveBook`) also re-check `kind = 'manual'` under the existing row lock, as defense in depth.

## API

All admin routes are in the existing `collections` admin group (`adminRole`, `requireBearer` on non-GET). Russian swagger annotations; regenerate `backend/docs`.

- `Rule` JSON: `{metric: "weighted_rating"|"most_read"|"most_wanted"|"trending", period_days: int|null, min_ratings: int|null, confidence: int|null}`. Responses always carry the normalized rule with defaults filled in. `min_ratings` and `confidence` are null unless the metric is `weighted_rating`.
- `Movement` JSON: `{direction: "up"|"down"|"same"|"new", by: int}` or `null`.
- Public `CollectionSummary` gains `rule: Rule|null` and `computed_at: RFC3339|null`. Public items become `{place, movement, book}` (`PublishedPage` also selects `previous_position`). `baseline_at` comes from the collection.
- `POST /admin/collections`: adds `kind?` and `rule?`. Validation messages:
  - `kind: must be manual or auto`;
  - `rule: required for automatic collections`;
  - `rule: only for automatic collections`;
  - `rule.metric: unknown metric`;
  - `rule.period_days: must be between 1 and 3650`;
  - `rule.min_ratings: must be between 1 and 10000`;
  - `rule.confidence: must be between 1 and 10000`.
- `PATCH /admin/collections/:id`: `kind` present → 400 `kind: cannot be changed`. `rule` is a whole replace. On an auto collection, if `rule`, `genre_id` or `size` actually changed, call `Recompute(reset=true)` after the update commits.
- `GET /admin/collections`: items gain `computed_at`.
- `GET /admin/collections/:id`: `AdminCollection` gains `rule`, `computed_at`, `baseline_at`; items gain `score: number|null` and `movement`.
- **New** `POST /admin/collections/:id/recompute` → 200 `AdminCollectionDetail`. Errors: 400 bad uuid; 404 `collection not found`; 422 `collection is not automatic`.
- `POST …/books`, `DELETE …/books/:bookId`, `PUT …/books/:bookId/position`, `POST …/books/paste/preview`, `POST …/books/bulk` on an auto collection → 422 `{"error":"collection is automatic"}`.

## Database

Forward-only migration `backend/migrations/000027_auto_collections.{up,down}.sql`:

```sql
ALTER TABLE curated_collections DROP CONSTRAINT curated_collections_kind_check;
ALTER TABLE curated_collections
    ADD CONSTRAINT curated_collections_kind_check CHECK (kind IN ('manual', 'auto')),
    ADD COLUMN rule_metric      VARCHAR(30)
        CHECK (rule_metric IN ('weighted_rating', 'most_read', 'most_wanted', 'trending')),
    ADD COLUMN rule_period_days INTEGER CHECK (rule_period_days BETWEEN 1 AND 3650),
    ADD COLUMN rule_min_ratings INTEGER CHECK (rule_min_ratings BETWEEN 1 AND 10000),
    ADD COLUMN rule_confidence  INTEGER CHECK (rule_confidence BETWEEN 1 AND 10000),
    ADD COLUMN computed_at      TIMESTAMPTZ,
    ADD COLUMN baseline_at      TIMESTAMPTZ,
    ADD CONSTRAINT curated_collections_auto_rule CHECK ((kind = 'auto') = (rule_metric IS NOT NULL));

ALTER TABLE curated_collection_items
    ADD COLUMN previous_position INTEGER CHECK (previous_position >= 1),
    ADD COLUMN score             DOUBLE PRECISION;

CREATE INDEX idx_curated_collections_kind ON curated_collections (kind);
```

- The inline CHECK of 000025 got the default name `curated_collections_kind_check`. Confirm it with `\d curated_collections` before writing the migration. If the name differs, the old CHECK would survive and inserting `auto` would fail, which the repository/integration test catches.
- Signal queries rely on the existing `ratings(book_id, user_id)` unique index, `shelf_items` PK and `book_genres` PK. Do not add rating/shelf indexes now (see Risks).
- The down file (convention only, **never run by agents**) deletes `auto` collections, drops the new columns and restores `CHECK (kind IN ('manual'))`.
- Apply with the usual migrate **up** only.
- GORM: add `RuleMetric *string`, `RulePeriodDays *int`, `RuleMinRatings *int`, `RuleConfidence *int`, `ComputedAt *time.Time`, `BaselineAt *time.Time` to `domain.Collection`, and `PreviousPosition *int`, `Score *float64` to `domain.CollectionItem`. Add the matching columns to the SQLite test schemas.

Aggregate queries (scope = published books, `book_genres.genre_id = ?` when the collection has a genre, period cut-off `since` when set):
- **weighted**: `SELECT r.book_id, b.title, b.ratings_count, SUM(r.value) AS rating_sum, COUNT(*) AS rating_count FROM ratings r JOIN books b ON b.id = r.book_id AND b.status = 'published' [JOIN book_genres bg ON bg.book_id = b.id AND bg.genre_id = ?] [WHERE r.updated_at >= ?] GROUP BY r.book_id, b.title, b.ratings_count`. C is computed in Go from the sums of all returned rows.
- **most_read / most_wanted**: the same shape over `shelf_items si JOIN shelves s ON s.id = si.shelf_id AND s.type = ?`, with `COUNT(*) AS signal_count` and `si.added_at >= ?`.
- **trending**: the ratings count in the period plus the reading-status shelf additions in the period, merged per book in Go (two queries) or with `UNION ALL` + `GROUP BY`.

## Frontend

`packages/api-client`:
- types `CollectionKind`, `AutoMetric`, `AutoRule`, `CollectionMovement` (`{direction, by}`);
- `rule` / `computed_at` on `CuratedCollection` and `AdminCuratedCollection`, `baseline_at` on the admin detail;
- `movement` on public and admin items, `score` on admin items, `computed_at` on admin list items;
- `kind` / `rule` in the create input and `rule` in the update input;
- `admin.collections.recompute(id)` through the bearer admin fetcher.

`apps/web` (FSD):
- `entities/curated-collection`:
  - `lib/movement.ts`: `formatMovement(m)` → `{text: '↑3'|'↓2'|'Новинка', label}` or null for `same`/null; aria labels «Поднялась на {by}», «Опустилась на {by}», «Новинка в подборке».
  - `lib/auto-caption.ts`: `autoCaption(rule, computedAt)` → «Составлена автоматически по {basis}{period}. Обновлено {date}.», with the date in `ru-RU` long form via the shared formatter if one exists.
  - `ui/movement-badge.tsx`.
- `views/collection-detail`: badge next to the place number; caption under the source line when `kind === 'auto'`. `views/collections` needs no change beyond types.

`apps/admin` (FSD):
- `entities/curated-collection/lib/labels.ts`:
  - kind labels «Ручная»/«Авто»;
  - metric labels;
  - `shortSnapshotWarning(shown, size)` → «Правилу соответствуют только {shown} из {size} книг» or null;
  - `reserveWarning` used for manual only;
  - a movement formatter. Keep it local to admin, do not import from web.
- `features/manage-collection`:
  - the form gains the «Тип» radio (create only; read-only label on edit) and the rule fields from the spec (metric select, period, min ratings, confidence shown only for the weighted rating, trending prefills 30);
  - `lib/collection-form.ts` maps the form to `{kind, rule}` and validates ranges client-side (UX only).
- `features/recompute-collection` (new): `useRecomputeCollection(id)` mutation; on success it sets/invalidates the detail query and the list.
- `widgets/collection-snapshot` (new):
  - «Пересчитано: …» / «Ещё не пересчитана», and the baseline line;
  - the «Пересчитать сейчас» button from `features/recompute-collection`;
  - a read-only snapshot table (place or state marker, movement, title + authors, score with 2 decimals for the weighted rating, an integer otherwise) and the short-snapshot warning.
- `views/collection-edit`:
  - render `widgets/collection-snapshot` when `kind === 'auto'`, otherwise `widgets/collection-books` as today (no view imports another view; the widget does not import `edit-collection-books` or `paste-collection-books`);
  - after a successful PATCH of an auto collection, invalidate the detail query so the reset snapshot shows.
- `views/collections`: «Тип» column; the warning per kind.
- Error mapping: 422 «collection is automatic» → «Книги автоматической подборки задаются правилом»; 422 «collection is not automatic» → «Пересчитать можно только автоматическую подборку».

## Existing Patterns

- Worker server and schedule wiring: `backend/cmd/worker/main.go` (`scheduleServerConfig`, `registerSchedules` with `scheduleRegistrar` seam, tested in `main_test.go`). Harvest tick task constructors/options are in `internal/modules/harvest/service/schedule.go`.
- Config flags and parsing: `internal/config/config.go` (`parseBoolEnv`, cron trimmed with a default) and `config_test.go` (`setHarvestScheduleEnv` style).
- Row-lock transaction: `curated/repository` `lock(tx, id)` and `AddBooks`.
- Read-time ranking: `curated/domain.Rank`, `ShownCount`.
- Nightly precompute job shape: `catalog/service` similarity job (`NewSimilarityJob`, `RecomputeTaskOptions`).
- Admin widgets/features: `widgets/collection-books`, `features/edit-collection-books`, `features/manage-collection`.

## Risks

- **Aggregate cost.** The weighted query scans `ratings` for the scope. At the current scale (thousands of rows) it takes milliseconds. If ratings grow to millions, add an index on `ratings(updated_at)` for period rules and consider moving «Пересчитать сейчас» to the worker. Synchronous recompute also holds the collection row lock for the query's duration, which only blocks edits of that one collection.
- **CHECK constraint name.** See Database. Verify it before writing the migration.
- **Baseline after a manual recompute** is the snapshot just replaced, so «↑/↓» can mean «since this morning». This is accepted and made visible through the baseline time on the admin page.
- **Small or young catalog.** A strict rule can leave a collection nearly empty. By decision there is no special-casing; the admin warning makes it visible, and the rule (min ratings, m) can be lowered.
- **Genre merge.** A collection whose genre is later merged keeps filtering by the alias id. If book links were repointed to the target, the snapshot may become empty. This is the same known limitation as feature 1, now with a visible effect (the short-snapshot warning).
- **Default-on schedule.** A new worker server and schedule start without configuration. They are harmless (no external calls), but call this out in the PR and in DEPLOYMENT.md.
- **typedRoutes.** No new routes. CI is still the build gate (no native `next build`).
