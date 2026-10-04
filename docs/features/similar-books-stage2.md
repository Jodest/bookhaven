# Similar Books, Stage 2: "Readers Also Read" + Daily Precompute

Status: implemented

GitHub issue: #217 "Похожие книги, этап 2: «Читатели также читали» и предрасчёт" (epic #223; sub-issues #224 backend, #225 devops). It builds on stage 1, [similar-books.md](similar-books.md) (#213, implemented). Branch `feat/similar-books-co-reading` from `main`.

## Goal

The "Похожие книги" block on the book page also uses a co-reading signal: books that the same readers read, are reading, or liked. Once a day a worker job recomputes a ranked top-20 per book into `book_similarities`. The ranking blends the stage-1 content score with the co-reading score. The co-reading weight grows with how many readers the book has. The public endpoint reads the precomputed list. Books without co-reading data fall back to the stage-1 live path unchanged. Only aggregates are used: a pair of books counts only when at least K distinct readers share it.

## Scope boundaries

**In scope:** migration `000014_book_similarities`, the co-reading aggregation, the blend, the batch job (catalog module), the asynq scheduler and a `similarity` queue in `cmd/worker`, a manual CLI `cmd/similarity`, the endpoint read path with fallback, the optional dev seed flag `-demo-readers`, tests, docs.

**Out of scope:** embeddings/pgvector, per-user personalization (#112), any API/frontend change (the response contract is identical), an admin UI or admin endpoint for the job, incremental/real-time updates, showing "N readers also read" counts in the UI.

## User Stories

- As a reader on the page of a book that people actually read, I see books other readers of this book also read or liked, mixed with same-author/same-genre books.
- As a reader on the page of a niche or new book, I still see the stage-1 content-based list.
- As a user, my shelves and ratings can never be inferred from the block: a recommendation only reflects a pattern shared by at least 3 readers.
- As an operator, the recomputation runs once a day without manual action, and I can run it by hand (dev, after a deploy, or after a data import).

## Acceptance Criteria

- [x] Migration `000014_book_similarities` (up/down) creates `book_similarities` as specified under Database. Down drops it.
- [x] A user-book "interaction" is defined exactly as in Business Rules: verified user, visible book, on a `read`/`reading` shelf or rated >= 4, and not rated <= 2.
- [x] A co-reading pair (a, b) is used only when `co(a,b) >= MinCoReaders` (3). Pairs below the threshold contribute nothing anywhere: no row, no score, no tie-break.
- [x] The co-reading score is cosine-normalized: `co / sqrt(n(a)·n(b))`. With equal co-counts, a bestseller ranks lower than a niche book.
- [x] The final score is `(1−w)·contentNorm + w·coScore` with `w` from Business Rules. The per-author cap (2) and the max of 20 still apply. The order is deterministic.
- [x] Stage-1 content scoring in the batch reuses the stage-1 code (`similarScore` and the cap selection). The weights and Jaccard logic are not duplicated.
- [x] A book gets rows in `book_similarities` only if it has at least one co-reading partner above K. Every other book has no rows and keeps the stage-1 live path.
- [x] The job replaces the whole table atomically in one transaction (DELETE + batched INSERT). Readers never see a half-written table. A failed run leaves the previous data intact.
- [x] `cmd/worker` runs an `asynq.Scheduler` that enqueues `similarity:recompute` daily at 03:30 UTC on the `similarity` queue. A dedicated `asynq.Server` with concurrency 1 processes it. The task uses `Unique`, `Timeout(30m)` and `MaxRetry(2)`. Worker shutdown stops the scheduler and all three servers.
- [x] `go run ./cmd/similarity` (and `/app/similarity` in the prod image) runs the same recomputation synchronously in-process and logs stats. It needs no Redis.
- [x] `GET /api/v1/books/:id/similar` keeps the exact stage-1 contract (status codes, shape, `limit` rules). If precomputed rows exist for the visible source book, they are returned in stored order. Otherwise the stage-1 cache/live path runs. The source visibility check (404) and the per-item `is_hidden` re-read still run on every request.
- [x] A failing precomputed read (DB error) returns 500, the same as other DB errors. It does not silently fall back.
- [x] Tests listed under Tests pass. Docker `go build/vet/test ./...` is clean. Swagger is unchanged.

## Business Rules

### Interactions

`U(x)` is the set of distinct users who have an interaction with book `x`. A user `u` has an interaction with book `x` when all of these hold:

1. `users.email_verified_at IS NOT NULL`. Unverified accounts are the cheapest way to fake co-reading, so they are excluded. Accounts that existed before #158 were backfilled as verified by 000011.
2. `books.is_hidden = false`. Hidden books are excluded on both sides of a pair.
3. At least one of these is true:
   - `x` is in a shelf of `u` whose `shelves.type IN ('read', 'reading')`. These are the system shelves; `shelf_type` is a Postgres enum and the per-user system shelves are unique via `idx_shelves_user_id_type`.
   - `ratings.value >= 4` for `(x, u)`. The scale is 1..5 (`SMALLINT CHECK 1..5`).
4. There is no `ratings` row for `(x, u)` with `value <= 2`. A book the user read and disliked is not a positive signal.

`want_to_read`, `abandoned`, `favorite` and custom shelves, lists, and reviews are not used. `reviews.rating` is ignored: `ratings` is the canonical per-user score that also feeds `books.rating`. The implementer should confirm whether creating a review also writes a `ratings` row; if it doesn't, keep reviews out anyway for this stage. Shelf privacy is not a factor because shelves are unconditionally public ([public-shelf-visibility.md](public-shelf-visibility.md)). Individual ratings are less exposed, which is why the K threshold applies to all signals.

**Heavy-user cap:** users with more than `MaxUserInteractions = 500` interactions are ignored entirely. These are bulk importers or bots. Each user contributes O(n²) pairs, so the cap also bounds the job cost. Users with fewer than 2 interactions contribute no pairs and are dropped early.

### Co-reading score

```
n(x)     = |U(x)|                       // after the filters above
co(a,b)  = |U(a) ∩ U(b)|
coScore  = co(a,b) / sqrt(n(a) · n(b))  // cosine, in (0, 1]
```

The pair is used only if `co(a,b) >= MinCoReaders = 3` (`K`, an exported constant). Cosine divides by the popularity of both books, so a bestseller that everyone has read is not "similar" to everything. A plain co-count would be. Cosine was chosen over Jaccard or lift because it is symmetric, bounded and well understood, and with K >= 3 it is stable enough at small counts.

### Blend

```
contentNorm = similarScore(...) / MaxContentScore            // MaxContentScore = AuthorWeight + GenreWeight + RatingBonusMax + PopBonusMax = 5.6
w           = CoWeightMax · min(1, n(a) / CoSupportFull)     // CoWeightMax = 0.6, CoSupportFull = 20
final       = (1 − w) · contentNorm + w · coScore
```

- `w` depends on the source book's reader count `n(a)`, which is the "data volume" for this book. With 3 readers, `w = 0.09`, so content dominates. With 20 or more readers, `w = 0.6`.
- `CoWeightMax < 1`, so same-author/genre relevance always keeps some weight and one strong co-reading pair cannot drown it out.
- Candidate set = stage-1 candidates (`ListSimilarCandidates`, the same pools) ∪ co-reading partners above K. A missing component is 0: co-only partners have `contentNorm = 0`, and content-only candidates have `coScore = 0`. The quality bonus for co-only partners is deliberately not computed, which avoids an extra query per book.
- Sort: `final DESC`, then `ratings_count DESC`, then `book_id` string ASC. Co-only partners take `ratings_count` from the interaction load (see Backend), so the tie-break needs no extra query.
- The per-author cap (2, counting the source's authors as in stage 1) is applied with the same greedy selection as stage 1. The result is truncated to `MaxSimilarLimit = 20`. Co-only partners need author IDs for the cap; these come from a single `book_authors` load (see Backend).

### Privacy

- Only the ranked ID list is exposed. Scores, `co` counts and reader counts are never returned by the API. `co_readers` is stored for diagnostics only.
- K = 3 means one reader alone, or two together, can never create or move a recommendation. It is a constant, not config, so it can't be lowered by accident. Raise it to 5 when the user base grows.
- Rows are visible in the DB to operators only, the same as `ratings`.

## API

No change. `GET /api/v1/books/{id}/similar?limit=N` keeps exactly the stage-1 contract ([similar-books.md](similar-books.md#api)). Swagger is not regenerated.

## Database

Migration `backend/migrations/000014_book_similarities`.

up:
```sql
CREATE TABLE book_similarities (
    book_id         UUID NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    similar_book_id UUID NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    position        SMALLINT NOT NULL,          -- 0-based rank after blend + author cap
    score           DOUBLE PRECISION NOT NULL,  -- final blended score
    content_score   DOUBLE PRECISION NOT NULL,  -- contentNorm (0 for co-only)
    co_score        DOUBLE PRECISION NOT NULL,  -- cosine (0 for content-only)
    co_readers      INTEGER NOT NULL,           -- co(a,b), 0 for content-only; diagnostics only, never exposed
    computed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (book_id, similar_book_id),
    CHECK (book_id <> similar_book_id)
);

-- FK cascade from books(id) via similar_book_id would otherwise scan the table.
CREATE INDEX idx_book_similarities_similar_book_id ON book_similarities(similar_book_id);
```
down:
```sql
DROP TABLE IF EXISTS book_similarities;
```

- The read query `WHERE book_id = ? ORDER BY position LIMIT 20` uses the PK prefix and sorts at most 20 rows, so no extra index is needed.
- The breakdown columns are kept because they make the blend debuggable with plain SQL (`SELECT ... ORDER BY position`). They cost little: at most 20 rows per book.
- The interaction query scans `shelf_items`/`shelves`/`ratings`/`users` once a day. No new index is needed for that: existing PKs, `idx_shelves_user_id`, the `ratings(book_id, user_id)` unique key, and the hash join on a full scan are fine.

## Backend design

Everything lives in the **catalog module**: stage 1 is there, and the job writes only catalog-owned data. Reading `shelves`/`ratings`/`users` tables in raw SQL from the catalog repository is a read-only aggregate, so it creates no import dependency on those modules. This is the same approach as the denormalized `books.rating`.

### Why the co-occurrence counting is done in Go

SQL only loads the filtered `(user_id, book_id)` interaction pairs. A **pure Go function** builds `U(x)`, counts pairs, applies K and computes cosine. Reasons:
- The tested function *is* the production logic. There is no Go "mirror" of SQL that could drift. Catalog tests are mock-based, with no Postgres in CI (stage 1 rationale).
- Cost: Σ_u n_u² map increments, where n_u <= 500 because of the heavy-user cap. Today (a handful of users, ~20–thousands of books) this takes milliseconds. Memory at 1M interactions is about 32 MB of UUID pairs plus a pair map bounded by Σ n_u². This is fine for the single VPS. Beyond that point, move the pair counting into SQL (a self-join of the interaction CTE with `GROUP BY ... HAVING COUNT(*) >= K`). The pure function stays the reference, and that is documented as the scaling path, not built now.

### Repository (`catalog/repository`)

To avoid growing `Repository` and breaking the three hand-written fakes again, add **separate interfaces** in `catalog/repository/similarity.go`, implemented by the same gorm repository:

```go
// Interaction is one filtered (user, book) positive signal.
type Interaction struct {
    UserID uuid.UUID
    BookID uuid.UUID
}

// SimilarityRow is one precomputed row.
type SimilarityRow struct {
    BookID, SimilarBookID uuid.UUID
    Position              int
    Score, ContentScore, CoScore float64
    CoReaders             int
}

// PrecomputedSimilarReader is used by the endpoint.
type PrecomputedSimilarReader interface {
    ListPrecomputedSimilar(ctx context.Context, bookID uuid.UUID, limit int) ([]uuid.UUID, error) // ORDER BY position
}

// SimilarityStore is used by the batch job only.
type SimilarityStore interface {
    ListInteractions(ctx context.Context) ([]Interaction, error)
    ListVisibleBookStats(ctx context.Context) ([]BookStat, error)                  // BookStat{ID uuid.UUID; RatingsCount int}, is_hidden=false
    ListBookAuthorIDs(ctx context.Context) (map[uuid.UUID][]uuid.UUID, error)  // all book_authors rows
    ReplaceSimilarities(ctx context.Context, rows []SimilarityRow) error
}
```

`ListInteractions` SQL. The enum is compared to string literals, which is fine in Postgres:

```sql
WITH pos AS (
    SELECT s.user_id, si.book_id
    FROM shelf_items si
    JOIN shelves s ON s.id = si.shelf_id
    WHERE s.type IN ('read', 'reading')
  UNION
    SELECT r.user_id, r.book_id FROM ratings r WHERE r.value >= 4
)
SELECT pos.user_id, pos.book_id
FROM pos
JOIN users u ON u.id = pos.user_id AND u.email_verified_at IS NOT NULL
JOIN books b ON b.id = pos.book_id AND b.is_hidden = FALSE
WHERE NOT EXISTS (
    SELECT 1 FROM ratings r2
    WHERE r2.user_id = pos.user_id AND r2.book_id = pos.book_id AND r2.value <= 2
);
```

`UNION` (not `UNION ALL`) dedups (user, book). The heavy-user cap and the "< 2 interactions" drop are applied in Go, in the pure function, so they are tested.

`ReplaceSimilarities`: `db.Transaction`, then `DELETE FROM book_similarities`, then `CreateInBatches(rows, 1000)`. All rows get the same `computed_at`, which is set in Go once per run. MVCC means concurrent readers see the old set until commit. A staging table plus rename was rejected because it needs DDL locks and buys nothing at this size (<= 20 × books rows).

`ListPrecomputedSimilar`: `SELECT similar_book_id FROM book_similarities WHERE book_id = ? ORDER BY position LIMIT ?`.

### Service (`catalog/service`)

**Refactor stage 1 without changing its behavior** (`similar.go`), so the batch reuses it:
- Extract the greedy cap loop from `rankSimilar` into `selectWithAuthorCap(items []rankedItem, limit int) []rankedItem`, where `rankedItem{ID uuid.UUID; Score float64; RatingsCount int; AuthorIDs []uuid.UUID}` is sorted by the shared comparator `lessRanked`.
- `rankSimilar` becomes: score with `similarScore`, sort with `lessRanked`, then `selectWithAuthorCap`. The existing `similar_test.go` must pass unchanged. That is the guard for "no behavior change".

**New `similarity.go`** (pure parts):

```go
const (
    MinCoReaders        = 3
    MaxUserInteractions = 500
    CoWeightMax         = 0.6
    CoSupportFull       = 20
    MaxContentScore     = AuthorWeight + GenreWeight + RatingBonusMax + PopBonusMax
)

type CoPartner struct { BookID uuid.UUID; Co int; Score float64 }

// coReading: pure. Applies the heavy-user cap and the <2 drop, counts pairs,
// keeps co >= minCo and computes cosine. Returns partners per book (both
// directions) plus n(x) per book.
func coReading(inter []repository.Interaction, minCo, maxPerUser int) (partners map[uuid.UUID][]CoPartner, readers map[uuid.UUID]int)

// blendSimilar: pure. Merges the stage-1 candidates (scored via similarScore)
// and co partners, applies w(n(a)), sorts with lessRanked, applies
// selectWithAuthorCap and returns at most max rows with breakdown.
func blendSimilar(src repository.SimilarSource, cands []repository.SimilarCandidate,
    partners []CoPartner, nSource int, ratingsCount map[uuid.UUID]int,
    authorIDs map[uuid.UUID][]uuid.UUID, max int) []repository.SimilarityRow
```

**`SimilarityJob`** (`similarity_job.go`):

```go
type SimilarityJob struct { repo repository.Repository; store repository.SimilarityStore; now func() time.Time; log *zap.Logger }
func NewSimilarityJob(repo repository.Repository, store repository.SimilarityStore, log *zap.Logger) *SimilarityJob
func (j *SimilarityJob) Run(ctx context.Context) (SimilarityStats, error)
```

`Run`:
1. `ListInteractions`, then `coReading(..., MinCoReaders, MaxUserInteractions)`.
2. If no book has partners, call `ReplaceSimilarities(nil)`. This clears stale rows, and every book uses stage 1. Return.
3. Load `ListVisibleBookStats` (for co-only `ratings_count`) and `ListBookAuthorIDs` once.
4. For each source book with partners, in a stable ID order: `GetSimilarSource` (skip on not-found, since the book was hidden mid-run), `ListSimilarCandidates` (the same stage-1 pools), then `blendSimilar`. Check `ctx.Err()` between books so the asynq timeout or shutdown aborts cleanly with no write.
5. `ReplaceSimilarities(allRows)`. Return stats: books with rows, total rows, interactions, users used, duration. Log them with zap.

The stage-1 queries run per source book, but only for books with co data. That is about 2 queries per book, which is acceptable once a day.

**asynq glue** (`similarity_task.go`, like `mailer/task.go`):
- `TaskTypeRecomputeSimilarities = "similarity:recompute"`, `SimilarityQueue = "similarity"`.
- `RecomputeTaskOptions()`: `asynq.Queue(SimilarityQueue), asynq.MaxRetry(2), asynq.Timeout(30*time.Minute), asynq.Unique(6*time.Hour)`. The empty payload is fine.
- `(*SimilarityJob).HandleTask(ctx, *asynq.Task) error` calls `Run(ctx)` and returns the error, which lets asynq retry.

**Endpoint** (`ListSimilarBooks`): add option `WithPrecomputedSimilar(r repository.PrecomputedSimilarReader)`. Existing `NewService` callers and fakes compile unchanged, and a nil reader means stage 1 only. The new order:
1. Normalize `limit`, then `GetSimilarSource`. This is unchanged and still gives the 404.
2. If the reader is set: `ids, err := ListPrecomputedSimilar(ctx, bookID, MaxSimilarLimit)`. A DB error returns `fmt.Errorf(...)`, which becomes a 500. If `len(ids) > 0`, use them and **skip Redis**.
3. Otherwise run the existing stage-1 path: Redis `similar:v1:` then live compute.
4. Unchanged: slice to `limit`, `GetBooksByIDs` and reorder, which drops books hidden since the job ran.

**Cache decision: keep `similar:v1:` unchanged, no bump, no invalidation.** The precomputed table is checked before Redis, so a warm v1 entry can never hide fresh stage-2 data. v1 entries keep their exact meaning, because the stage-1 content ranking is unchanged and only used for books without co data. When a book loses its co data, its v1 entry (at most 6h old) is still a correct stage-1 answer. The precomputed read is a single PK-prefix query with at most 20 rows, so it isn't cached.

### Worker (`cmd/worker/main.go`)

- Build `catalogRepo := catalogrepository.NewGormRepository(db)` (use the existing constructor name) and `job := catalogservice.NewSimilarityJob(catalogRepo, catalogRepo, logger)`.
- Add a **third `asynq.Server`**: `Queues: {similarity: 1}`, `Concurrency: 1`, mux `TaskTypeRecomputeSimilarities → job.HandleTask`. This is the same pattern as email: a 30-minute batch never blocks harvest or email, and harvest's throttling assumptions are untouched.
- Add `asynq.NewScheduler(redisOpt, &asynq.SchedulerOpts{Location: time.UTC, Logger: zapAsynqLogger{logger}})` and `Register("30 3 * * *", asynq.NewTask(TaskTypeRecomputeSimilarities, nil), RecomputeTaskOptions()...)`. Call `scheduler.Start()`, which is non-blocking. On shutdown, call `scheduler.Shutdown()` first, then the servers. If start fails, shut down the already-started servers before `Fatal`, as the email server does.
- **Single-instance scheduling:** the scheduler lives in the worker because prod runs exactly one worker container (`container_name: bookhaven-worker`, no replicas). A separate scheduler process would mean a sixth container for one cron line. As defense in depth, `asynq.Unique(6h)` means that if two schedulers ever run (a scaled worker, or dev and prod sharing Redis by mistake), the second enqueue is rejected as a duplicate. Document "do not scale the worker beyond 1 without moving the scheduler".
- **No run on startup.** `air` restarts the dev worker on every save, so a startup run would recompute constantly. In prod, the first nightly run, or a manual run right after deploy, is enough. An empty table already means "stage 1 for everything", which is correct.
- Update the package doc comment to mention the similarity server and scheduler.

### Manual trigger: `cmd/similarity/main.go`

A small CLI with the same config/DB bootstrap as `cmd/seed` and `cmd/worker`. It calls `NewSimilarityJob(...).Run(ctx)` **synchronously** with a 30-minute context, prints the stats, and exits non-zero on error. No Redis or worker is needed, so it works with nothing else running. Add `go build -o /app/bin/similarity ./cmd/similarity` and the matching `COPY` to `backend/Dockerfile`, as for `harvest`.
- Dev: `docker compose exec backend go run ./cmd/similarity`
- Prod: `docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/similarity backend`

An admin-only endpoint was rejected: it would add authz, routing and Swagger surface for an ops action.

### Dev data: `cmd/seed -demo-readers` (cheap, include)

With 2 seed users the table would always be empty. Add the flag `-demo-readers`, which is subject to the existing production guard (`-force`). It creates 12 verified demo users `reader01..reader12@demo.bookhaven.local` with `password_hash NULL`, so they can't log in, via `FirstOrCreate` (idempotent). Their system `read` shelves are filled with deterministic, overlapping subsets of the seeded books: for example, readers 1–6 get the Russian-classics cluster and readers 7–12 the other cluster, with a 2–3 book overlap. Some get ratings of 4–5, and one reader rates one book 1 to exercise the dislike rule. Resolve books by seed slug, and skip missing ones. Then run `go run ./cmd/similarity`. Shelf creation must follow the existing shelves materialization semantics (system shelf per type; partial unique index), using `FirstOrCreate` on `(user_id, type)`. Extend `cmd/seed/main_test.go` only if the existing test style covers flag parsing; otherwise rely on manual verification.

## Existing Patterns

- Stage 1 scoring/cap and the option-based `NewService`: `backend/internal/modules/catalog/service/similar.go`.
- Task type/queue/options plus a pure handler: `backend/internal/pkg/mailer/task.go` and `task_test.go`. A per-queue `asynq.Server`: `backend/cmd/worker/main.go` (email server).
- CLI bootstrap and production guard: `backend/cmd/seed/main.go`. Prod binary packaging: `backend/Dockerfile` (`harvest`).
- Visibility filter `is_hidden = false` and raw SQL with `db.Raw(...).Scan`: `catalog/repository/repository.go` (`ListSimilarCandidates`).

## Tests

Backend (`go test ./...` via docker compose):
- `catalog/service/similarity_test.go`, pure `coReading`:
  - The K threshold: co = 2 gives no partner, co = 3 gives a partner in both directions.
  - Cosine values on a hand-computed example.
  - A bestseller read by everyone ranks below a niche co-read book with equal co.
  - A user with more than 500 interactions is ignored entirely, including in `n(x)`.
  - A user with 1 interaction contributes nothing.
  - Duplicate interactions don't double count.
  - Empty input gives empty maps.
- Pure `blendSimilar`:
  - `w` grows with `n(a)` and is capped at 0.6 (n = 3 gives 0.09, n = 20 and n = 100 give 0.6).
  - Content-only and co-only candidates both appear with zero components.
  - A candidate that is in both sets sums both components.
  - The per-author cap still holds (co-only partners use the `authorIDs` map).
  - Truncation to 20.
  - Deterministic tie-break.
  - Positions are 0..n−1.
  - The source book is never in its own list.
- `similar_test.go` (stage 1) passes unchanged after the `selectWithAuthorCap`/`lessRanked` extraction.
- `SimilarityJob.Run` with a fake `Repository` + fake `SimilarityStore`:
  - No partners anywhere means `ReplaceSimilarities(nil)` is called.
  - Only books with partners get stage-1 queries.
  - A source not found mid-run is skipped.
  - A store/repo error returns an error and does not call `ReplaceSimilarities`.
  - A cancelled ctx aborts before the write.
- `HandleTask`: it calls `Run` and propagates the error. `RecomputeTaskOptions` has the expected queue, retry and timeout (asynq exposes options via `task` / `asynq.Option.Type()`, like `mailer/task_test.go`).
- `ListSimilarBooks` with a fake reader:
  - Precomputed IDs win, and both Redis `Get` and `ListSimilarCandidates` are not called.
  - An empty precomputed list falls back to the cache/live path.
  - A reader error gives an error, not a fallback.
  - A nil reader behaves exactly as stage 1.
  - A hidden ID in the precomputed list is dropped.
  - The 404 for a hidden source happens before the precomputed read.
- Scheduler registration: extract `registerSchedules(s scheduleRegistrar) error` in `cmd/worker`, with a one-method interface `Register(cronspec string, task *asynq.Task, opts ...asynq.Option) (string, error)`. Test that it registers `"30 3 * * *"` with `similarity:recompute`.

Manual (record in the PR), against the dev Postgres:
- `migrate up`/down/up for 000014, and `\d book_similarities`.
- `go run ./cmd/seed -demo-readers`, then `go run ./cmd/similarity`. Check that the stats are non-zero and that `SELECT ... ORDER BY position` for a classic shows co-read books with `co_readers >= 3` only.
- `curl /api/v1/books/<id>/similar` for a book with rows (it differs from stage 1) and for one without (stage-1 list; the Redis key `similar:v1:` is set).
- Hiding a precomputed partner removes it from the response immediately.
- The worker logs the scheduler start. `asynq` CLI or Redis shows the scheduler entry. Temporarily register `"* * * * *"` locally (not committed) to see one run end to end.

## Documentation

- This contract: on completion, set Status to `implemented`, tick the criteria and add Implementation notes.
- `docs/features/similar-books.md`: add a line under the header: "Stage 2 (co-reading + precompute): see [similar-books-stage2.md](similar-books-stage2.md)."
- `docs/ROADMAP.md`, epic #218:
  - Move stage 1 (#213, #198) to Completed (merged in #221/#222).
  - Mark stage 2 (#217) "in progress (see [docs/features/similar-books-stage2.md](features/similar-books-stage2.md))", then move it to Completed on merge.
  - Remove the now-done Proposed line for #213 if it is still listed.
- `docs/DEPLOYMENT.md`:
  - Worker section: it now also runs the daily similarity recompute (03:30 UTC, `similarity` queue) via an in-process asynq scheduler. **Do not run more than one worker replica** without moving the scheduler.
  - The manual recompute command (prod `run --rm --entrypoint /app/similarity`), and that running it once right after the first deploy of this feature is recommended.
- `README.md` (dev commands section, if present): `go run ./cmd/similarity` and `go run ./cmd/seed -demo-readers`.
- No Swagger or api-client changes.

## Risks

- **Prod worker JWT_SECRET issue (deferred):** the worker may be crash-looping in prod because of the known config issue. If it is, the scheduler never fires and every book silently stays on stage 1. Check the worker is healthy before relying on the nightly job, or run `/app/similarity` manually.
- **Scheduler duplication** if the worker is ever scaled. This is mitigated by `asynq.Unique(6h)` and documented. The full-replace job is idempotent anyway, so a duplicate run only wastes CPU.
- **Small-K re-identification:** with K = 3, someone who knows two readers' shelves could, in theory, infer a third reader's book. Shelves are already public, so the residual risk concerns ratings only. It is accepted at the current scale. Raise `MinCoReaders` to 5 as the user count grows.
- **Cost growth:** Σ n_u² in Go memory. It is bounded by the 500-interaction cap and fine for years at the current scale. The SQL-side pair counting is the documented scaling path. Log the job duration and pair count to see this coming.
- **Staleness up to 24h** for co-read ordering. Visibility is still re-checked live, so hidden books never leak. Content changes (new books, genre edits) for books *with* rows also wait up to 24h, versus 6h for stage-1 books. Accepted.
- **Full-table DELETE + INSERT once a day** creates dead tuples equal to the table size. This is trivial at <= 20 × books rows, and autovacuum handles it.
- **Stage-1 refactor regression:** extracting `selectWithAuthorCap`/`lessRanked` must not change stage-1 output. The unchanged `similar_test.go` is the guard.
- **Catalog reading other modules' tables** (`shelves`, `ratings`, `users`) in raw SQL. This is a read-only aggregate with no Go import dependency, which is acceptable. If those schemas change, this query must be updated, so mention it in a comment on `ListInteractions`.
- **Demo readers in a shared/staging DB:** `-demo-readers` is guarded like the rest of the seed. Its emails use a non-routable `.local` domain, and with no password nobody can log in as them.

## Implementation notes

- **Reviews write `ratings`: confirmed.** Creating or updating a review upserts the `ratings` row for `(book, user)` in the same transaction (`reviews/repository.upsertRatingInTx`) and recalculates `books.rating`. So a review's score already reaches the interaction query through `ratings`; `reviews.rating` itself is not read.
- **Signature deviations (minimal):**
  - `blendSimilar` takes the source book ID as an extra first argument (`srcID`). The rows need `BookID`, and it lets the function drop the source from its own list defensively.
  - `coReading` returns a third value, `usersUsed`, for the run stats ("users used").
  - `SimilarityRow` has a `ComputedAt` field; the job sets one timestamp per run.
  - `blendSimilar`'s last parameter is named `maxRows` (not `max`, to avoid shadowing the builtin).
- **Users with fewer than 2 interactions** are dropped entirely, as the contract says: they count neither in pairs nor in `n(x)`. Same for users above `MaxUserInteractions` (500 is still allowed).
- **Stage-1 refactor:** `rankSimilar` now uses the extracted `rankedItem`, `lessRanked`, `sortRanked`, `selectWithAuthorCap`, `authorSet` and `isAuthorMatch`. `similar_test.go` is unchanged and passes.
- **Repository wiring:** `NewGormRepository` still returns the `Repository` interface (existing callers unchanged). The new `NewGormSimilarityRepository(db) *GormRepository` returns the concrete type that also implements `SimilarityStore` and `PrecomputedSimilarReader` (compile-time assertions in `catalog/repository/similarity.go`). `cmd/api` passes it via `WithPrecomputedSimilar`; `cmd/worker` and `cmd/similarity` pass it as both `repo` and `store`.
- **Task helpers:** `NewRecomputeSimilaritiesTask()` builds the empty-payload task; `registerSchedules` (cmd/worker) uses it with `RecomputeTaskOptions()`. The worker logs `asynq scheduler started` with the cron spec.
- **`-demo-readers`** (cmd/seed/demo_readers.go) runs in one transaction. It creates the system shelves with the same `INSERT ... ON CONFLICT (user_id, type) WHERE type <> 'custom' DO NOTHING` as `shelves.EnsureDefaultShelves` (instead of a gorm `FirstOrCreate`, so the semantics are exactly the canonical ones). Ratings are upserted and `books.rating`/`ratings_count` are recalculated for the rated books, so the denormalized values stay consistent. Data: 12 readers, 48 "read" shelf items, 8 ratings (reader06 rates `war-and-peace` 1; reader11 rates `fathers-and-sons` 5 without shelving it). `cmd/seed/main_test.go` was not extended (it only covers the production guard).
- **Local verification (dev DB):** `migrate up` 13 -> 14; up/down/up checked on a throwaway database and the down migration inside `BEGIN ... ROLLBACK`. `cmd/similarity` after `-demo-readers`: interactions=48, users_used=12, books_with_co=9, co_pairs=11, rows=123; no row has `co_readers` 1..2. `war-and-peace` (with rows) returns the blended order (co-read `crime-and-punishment`/`brothers-karamazov` move above the content-only `fathers-and-sons`) and sets no `similar:v1:` key; `resurrection` (no rows) returns the stage-1 list and sets `similar:v1:`. Hiding a precomputed partner drops it from the response immediately. The worker logs the scheduler start and three servers; a manually enqueued `similarity:recompute` was processed by the worker, and a second enqueue was rejected as a duplicate (`task already exists`, Unique).
