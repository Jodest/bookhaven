## Context

Issue #510 is part 3 of «Книга или экранизация?». Part 2 (#509, archived `2026-10-20-add-adaptation-verdict-votes`) added the table `adaptation_votes`, created by migration `000035`:
- PK `(user_id, book_id, screen_work_id)`;
- `value` is `book`, `adaptation` or `same`;
- `updated_at` changes only when the value changes;
- composite FK to `book_screen_works` ON DELETE CASCADE, and indexes `(book_id, screen_work_id)` and `(updated_at)`.

A visible link is a `book_screen_works` row with `NOT hidden`. The verdict endpoint's minimum is the domain constant `screenworks/domain.MinVerdictVotes = 5`.

Automatic collections were added by archived change `2026-10-14-add-auto-collections`, migration `000027`:
- `curated/domain/auto.go`: `Rule`, `Normalize`, `AutoRow` and `RankAuto`.
- `curated/repository/auto.go`: `txSource.AutoRows` builds the aggregates per metric, and `Recompute` writes the snapshot under the row lock.
- `curated/service/service.go`: `recompute` turns `period_days` into `since`.
- `RecomputeAll` is run nightly by the worker. It needs no change.

The rule columns are `rule_metric`, `rule_period_days`, `rule_min_ratings` and `rule_confidence`. `rule_metric` has an inline CHECK listing the 4 current metrics. The snapshot rows are `curated_collection_items`, with `score` and `previous_position`.

## Decisions

1. **Source adaptation = most counted votes.** This is the owner's decision. It is computed from counted votes, so the period and hidden links are already applied.
   - Tie-break: the public adaptation order (`year ASC NULLS LAST, title, id`), the same ORDER BY as `ListPublic` of screenworks. A reader sees the source as the first of the tied adaptations on the book page. It is deterministic and needs no extra signal.
   - There is no fallback to another adaptation when the source fails the minimum. The source has the most votes, so no other adaptation would pass.
2. **Per-adaptation minimum = rule `min_ratings`, default 5, floor 5.** The rule column is reused, so there is no rule migration and no API rename. For verdict metrics it means "minimum votes of the source adaptation". The floor of 5 matches the verdict endpoint, which hides per-value counts below 5. A collection then never reveals a split that the book page would hide. `confidence` (m) is reused too, default 10.
3. **Leader gate.** The metric's value must strictly lead the source adaptation's votes. This matches the book-page verdict, where a tie reads «Мнения разделились». Without the gate, a small «Книги, которые лучше фильма» list could show books whose adaptation won.
4. **Score.** The score is `(k + m·C) / (v + m)`, stored as 0-1 in `score`. `same` votes count in v but never in k. C is computed over every counted vote in scope: all visible links of in-scope published books, including non-source adaptations and books below the minimum. This mirrors how `weighted_rating` computes C.
5. **Period.** The filter is `adaptation_votes.updated_at >= since`. This is consistent with ratings (`r.updated_at`): "the time the signal was given". Re-sending the same value does not bump `updated_at`, which was already guaranteed by #509. Retracted votes are deleted, so they never count. A vote whose author has since left «Прочитано» still counts, because #509 keeps such votes.
6. **Tie order of books.** Score desc, then source votes desc, then `books.ratings_count` desc, title and id. The extra key gives a stable order and prefers the better-supported share.
7. **The snapshot stores the source.** Three nullable columns go on `curated_collection_items`: `source_screen_work_id`, `source_votes` and `source_share`. The row then names the adaptation, and its figures stay frozen with the place between computations, as the spec requires. Reading the figures live would let the line drift away from the stored score.
8. **Read-time visibility of the source.** Public and admin item reads LEFT JOIN `book_screen_works` (`NOT hidden`) and `screen_works`. A hidden or deleted link gives `adaptation: null`, but the place is kept. This prevents showing a link that an admin hid as wrong.
9. **Basis caption.** One shared basis for both metrics, «голосам читателей «Книга или экранизация?»». The collection title already says which side wins.

## API

No new endpoints and no authorization change. Public routes stay open. Admin routes keep the `admin` role, plus a bearer token for writes.
- Rule: `metric` adds `"book_beats_adaptation" | "adaptation_beats_book"`. `Normalize`:
  - fills `min_ratings` 5 and `confidence` 10 for verdict metrics;
  - validates `min_ratings` 5-10000 for them and returns `&RuleError{"rule.min_ratings: must be between 5 and 10000"}`;
  - the weighted rating stays at 1-10000.
  `domain.Rule.Equal` is unchanged.
- `GET /api/v1/collections/{slug}`: each `placedBookResponse` gains `adaptation`:
  ```json
  {"id":"<uuid>","title":"Война и мир","year":1967,"kind":"film","votes_count":40,"share":0.9}
  ```
  or `null`. The field is always present (no omitempty).
- `GET /api/v1/admin/collections/{id}` (and the responses of POST/PATCH/recompute that return the detail): each `adminItemResponse` gains the same `adaptation`.
- Update the swagger annotations and regenerate `backend/docs`.

## Database

`backend/migrations/000036_verdict_collections.up.sql`:
```sql
ALTER TABLE curated_collections DROP CONSTRAINT curated_collections_rule_metric_check;
ALTER TABLE curated_collections ADD CONSTRAINT curated_collections_rule_metric_check
    CHECK (rule_metric IN ('weighted_rating', 'most_read', 'most_wanted', 'trending',
                           'book_beats_adaptation', 'adaptation_beats_book'));

ALTER TABLE curated_collection_items
    ADD COLUMN source_screen_work_id UUID REFERENCES screen_works(id) ON DELETE SET NULL,
    ADD COLUMN source_votes          INTEGER CHECK (source_votes >= 1),
    ADD COLUMN source_share          DOUBLE PRECISION CHECK (source_share BETWEEN 0 AND 1);
```
- The inline CHECK from `000027` gets Postgres's default name `curated_collections_rule_metric_check`. Before writing the migration, the developer MUST confirm the name read-only, e.g. `docker compose exec postgres psql -U <user> -d <db> -c '\d curated_collections'`.
- The down file drops the three columns and restores the 4-metric CHECK. It exists as a file only and is never run.
- No index is needed: items are always read by `collection_id`.
- Apply only with `docker compose exec backend go run ./cmd/migrate up`.

Aggregate query (new `txSource.verdictRows`; use `SUM(CASE …)`, not `FILTER`, so the SQLite repository tests run):
```sql
SELECT v.book_id, v.screen_work_id, b.title, b.ratings_count, w.title AS work_title, w.year AS work_year,
       COUNT(*) AS votes,
       SUM(CASE WHEN v.value = 'book' THEN 1 ELSE 0 END)       AS book_votes,
       SUM(CASE WHEN v.value = 'adaptation' THEN 1 ELSE 0 END) AS adaptation_votes
FROM adaptation_votes v
JOIN book_screen_works l ON l.book_id = v.book_id AND l.screen_work_id = v.screen_work_id AND NOT l.hidden
JOIN screen_works w ON w.id = v.screen_work_id
JOIN books b ON b.id = v.book_id AND b.status = 'published'
[JOIN book_genres bg ON bg.book_id = b.id AND bg.genre_id = ?]
[WHERE v.updated_at >= ?]
GROUP BY v.book_id, v.screen_work_id, b.title, b.ratings_count, w.title, w.year
```
Reuse `txSource.scope` for the books and genre joins.

## Backend code shape

- `domain/auto.go`:
  - constants `MetricBookBeatsAdaptation` and `MetricAdaptationBeatsBook`, `DefaultMinVerdictVotes = 5` (comment: keep equal to screenworks `MinVerdictVotes`; the curated module MUST NOT import screenworks), and `IsVerdictMetric(m)`;
  - `VerdictRow {BookID, Title, RatingsCount, ScreenWorkID, WorkTitle, WorkYear *int, Votes, BookVotes, AdaptationVotes int64}`;
  - `ScoredBook` gains `Source *VerdictSource {ScreenWorkID uuid.UUID; Votes int; Share float64}`, nil for other metrics.
  - A pure `RankVerdicts(rows []VerdictRow, rule Rule, limit int) []ScoredBook` implements the spec: C, source pick with the tie-break, minimum, leader gate, score and order. Keep `RankAuto` untouched.
- `repository`:
  - `AutoSource` gains `VerdictRows(ctx, c, since)`;
  - `Recompute` writes `SourceScreenWorkID`, `SourceVotes` and `SourceShare` from `ScoredBook.Source`, with new pointer fields on `domain.CollectionItem`;
  - `PublishedPage` and `AdminItems` return the source fields joined with visibility and the work's `title`, `year` and `kind`, through a new `SourceAdaptation` struct on `PublishedBook` and `AdminItem`.
- `service.recompute`: dispatch on `IsVerdictMetric` to `VerdictRows` + `RankVerdicts`, else the existing path. Pass the source through `PublicItem`/`AdminDetailItem` to transport.

## Frontend

- `packages/api-client`:
  - `AutoMetric` adds the two values;
  - new `CollectionSourceAdaptation {id, title, year: number | null, kind, votes_count, share}`;
  - `adaptation: CollectionSourceAdaptation | null` on `CuratedCollectionPlace` and on the admin detail item type, with the snake→camel mapping that the place items already use;
  - `AutoRule` doc comments are updated (`min_ratings`/`confidence` also apply to the verdict metrics).
- `apps/web`:
  - `entities/curated-collection/lib/auto-caption.ts` adds both metrics to `BASIS`;
  - new pure `lib/adaptation-line.ts` `adaptationLine(metric, adaptation)` returns the spec string or null. It uses `shared/lib/plural-ru` (голос/голоса/голосов) and `Math.round(share * 100)`. Tests cover both metrics, missing year, and 1/2/5/21 plurals;
  - `views/collection-detail/ui/collection-detail-view.tsx` renders the line under the rating when `collection.rule` is a verdict metric and the item has an adaptation. Extend `collection-detail-view.test.tsx`.
- `apps/admin`:
  - `entities/curated-collection/lib/labels.ts`: `METRIC_LABELS` gets the two options. `formatScore` returns `${Math.round(score*100)}%` for the verdict metrics. A new `adaptationSourceLabel(adaptation)` returns «Экранизация: «…» (year) · N голосов». Extend `labels.test.ts`;
  - `features/manage-collection/lib/collection-form.ts`:
    - `DEFAULT_MIN_VERDICT_VOTES = '5'`;
    - `metricPatch` resets `min_ratings` to the new metric's default when switching between the weighted rating and the verdict metrics;
    - `validateRule` checks «Минимум голосов: от 5 до 10000»;
    - `formToRule` sends `min_ratings`/`confidence` for the verdict metrics too;
    - `collectionToForm` keeps loading stored values.
    Extend `collection-form-auto.test.ts`;
  - `features/manage-collection/ui/collection-form.tsx`: for verdict metrics, label the minimum «Минимум голосов» and show the hint from the spec;
  - `widgets/collection-snapshot/ui/collection-snapshot.tsx`: renders `adaptationSourceLabel` under the authors. Extend its test.
- FSD: the web view imports only the entity lib. The admin widget imports only the entity. No cross-feature imports.

## Existing Patterns

- Metric plumbing end to end: archived change `2026-10-14-add-auto-collections` (design and tasks) and `repository_auto_test.go` / `service_auto_test.go` / `auto_test.go`.
- Bayesian score: `RankAuto` weighted branch (C over all in-scope rows).
- Visible link and public adaptation order: screenworks `ListPublic` / `ListVerdicts`.
- Plural forms and the percent rounding of a verdict: web `entities/screen-work/lib/verdict-label.ts`.

## Risks

- **Constraint name**: if `000027`'s CHECK is named differently, `DROP CONSTRAINT` fails and the migration aborts, which is safe. Confirm the name first (see Database).
- **Sparse data**: a young site has few books with 5+ votes on one adaptation, so the collections will be short. The existing «Правилу соответствуют только …» warning covers this. It is the expected behavior and needs no special handling.
- **Two collections, one adaptation pool**: a book can appear in neither list (no leader, or a «Одинаково» majority), but never in both. The strict leader gate makes the two lists mutually exclusive.
- **Source deleted**: a work can only be deleted without links, and link deletion cascades the votes. `ON DELETE SET NULL` keeps the snapshot row valid, and the row then shows no line.
- **Duplicated constant**: `DefaultMinVerdictVotes` duplicates screenworks `MinVerdictVotes`. Add a comment on both sides.
