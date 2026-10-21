# Tasks

Safety rules for every group:
- Never run destructive DB commands: no `migrate down`, no `docker compose down -v`, no DROP/TRUNCATE against a database.
- Apply migrations only with `docker compose exec backend go run ./cmd/migrate up`.
- Run `gofmt -w` only on the files you changed, never on the whole backend.
- Never run a native `next build`: CI is the typedRoutes gate.

## 1. Backend

- [x] 1.1 Confirm the name of the `rule_metric` CHECK read-only (`\d curated_collections` via `docker compose exec postgres psql …`). Then add forward-only `backend/migrations/000036_verdict_collections.{up,down}.sql` as in design.md. That means a widened metric CHECK, plus `source_screen_work_id` (FK `screen_works` ON DELETE SET NULL), `source_votes` and `source_share` on `curated_collection_items`. Do not edit earlier migrations. Verify by applying `up` with the command above.
- [x] 1.2 Domain (`curated/domain`):
  - the metric constants, `IsVerdictMetric` and `DefaultMinVerdictVotes = 5`, with a comment cross-referencing screenworks `MinVerdictVotes`, and the same comment added there;
  - `Normalize`: verdict defaults (5 / 10) and the 5-10000 floor with «rule.min_ratings: must be between 5 and 10000»;
  - `VerdictRow`, `VerdictSource` on `ScoredBook`, and the pure `RankVerdicts`;
  - new `CollectionItem` source fields.
  Unit-test in `auto_test.go` every «Verdict score» scenario (the «Война и мир» example gives 0.884 with C = 0.82, equal votes → older year, no year last, minimum, leader gate including a tie and a «same» majority, `adaptation_beats_book` mirror), the tie order (score, then source votes, then ratings count), and the `Normalize` defaults and errors.
- [x] 1.3 Repository:
  - `txSource.VerdictRows`: visible links, published books, genre scope via `scope`, `updated_at >= since`, and `SUM(CASE …)` so SQLite works;
  - `Recompute` writes the source fields;
  - `PublishedPage` and `AdminItems` return the source adaptation (id, title, year, kind, votes, share) only while the link is visible.
  Extend `repository_auto_test.go`: hidden-link votes excluded, period filter, genre filter, a source hidden after the snapshot reads as no adaptation with the place kept, and the source fields persisted.
- [x] 1.4 Service: `recompute` dispatches verdict metrics to `VerdictRows` + `RankVerdicts`. Pass the source through the public and admin item models. Extend `service_auto_test.go`: create with `book_beats_adaptation` defaults, an invalid minimum gives 400, the immediate snapshot, and `RecomputeAll` covering a verdict collection.
- [x] 1.5 Transport: `adaptation` (always present, null when absent) on `placedBookResponse` and `adminItemResponse`, per the design.md API. Extend `handler_auto_test.go` for the JSON shape. Check that a moderator still gets 403 and a cookie-only write 401 for a verdict-metric create. Update the swagger annotations and regenerate `backend/docs`.
- [x] 1.6 Verify from `backend/`: `go build ./...`, `go vet ./...` and `go test ./...`.

## 2. Frontend (api-client and web)

- [x] 2.1 `packages/api-client`: `AutoMetric` gets the two values. Add the `CollectionSourceAdaptation` type and `adaptation` on `CuratedCollectionPlace` and on the admin detail item, with the mapping. Update the `AutoRule` comments. Verify with `tsc --noEmit`.
- [x] 2.2 `apps/web` `entities/curated-collection`:
  - `BASIS` gets both metrics, and `auto-caption.test.ts` gains the verdict caption case;
  - new pure `lib/adaptation-line.ts` with tests: both metric texts, no year, 1/2/5/21 plurals, 0.884 → 88 rounding, null when no adaptation.
  Export it from the entity index.
- [x] 2.3 `apps/web` `views/collection-detail`: render the adaptation line under the rating. Extend `collection-detail-view.test.tsx` with a verdict place and a `weighted_rating` place, which has no line.
- [x] 2.4 Verify in `apps/web`: `tsc --noEmit`, `next lint` and the web unit tests.

## 3. Frontend (admin)

- [x] 3.1 `entities/curated-collection/lib/labels.ts`:
  - the two `METRIC_LABELS` entries;
  - `formatScore` shows a whole percent for verdict metrics;
  - `adaptationSourceLabel`.
  Extend `labels.test.ts`.
- [x] 3.2 `features/manage-collection`:
  - `collection-form.ts`: the verdict default 5, the `metricPatch` reset between families, «Минимум голосов: от 5 до 10000» validation, and `formToRule` sending min/confidence for the verdict metrics;
  - `collection-form.tsx`: the «Минимум голосов» label and hint for the verdict metrics.
  Extend `collection-form-auto.test.ts` and the edit-view test (verdict form shows «Минимум голосов» 5 and no «Минимум оценок»).
- [x] 3.3 `widgets/collection-snapshot`: show «Экранизация: «…» (year) · N голосов» under the authors and the percent score. Extend `collection-snapshot.test.tsx`.
- [x] 3.4 Verify in `apps/admin`: `tsc --noEmit`, `next lint` and the admin unit tests.

## 4. Docs

- [x] 4.1 `docs/admin-guide/collections.md`: describe the two verdict metrics in the automatic-collections part. Cover the source adaptation (most votes, ties by the book-page order, hidden links ignored), the minimum of 5 votes and the leader rule, m, the period by vote time, and why a list can be short. Include a suggested setup, e.g. «Книги, которые лучше фильма» = `book_beats_adaptation`, size 50, all time.
- [x] 4.2 `docs/ROADMAP.md`: remove #510 from the «Queued 2026-10-20 …» list; drop the paragraph if it becomes empty. Add a `- [x]` done entry after the #519 entry, in the same style as #509: epic/issue numbers, the metrics, scoring by the most-voted visible adaptation, the minimum of 5, migration `000036_verdict_collections`, and OpenSpec change `add-adaptation-verdict-collections`. Change «Next: #510» in the #519 entry so it reads as complete.
