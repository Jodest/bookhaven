# Proposal

GitHub issue: #510 (part 3 of 3 of «Книга или экранизация?», builds on #509 / archived change `2026-10-20-add-adaptation-verdict-votes`).

## Why

Readers' «Книга или экранизация?» votes are only visible on one book page at a time. Two automatic collections, «Книги, которые лучше фильма» and «Экранизации, которые превзошли книгу», turn them into discovery pages that the existing nightly snapshot keeps up to date.

## What Changes

- Two new automatic collection metrics, `book_beats_adaptation` and `adaptation_beats_book`.
- A book is scored by its **source adaptation**: the visible adaptation with the most counted votes. Ties go to the adaptation listed first on the book page: oldest year first, works without a year last, then title, then id.
- The score is the Bayesian-smoothed share of «Книга лучше» (or «Экранизация лучше») votes, `(k + m·C) / (v + m)`, the same shape as `weighted_rating`.
- A book is ranked only when its source adaptation has at least the rule's minimum votes (default 5, the same as the verdict endpoint's `min_votes`; it cannot be set below 5) and the metric's value strictly leads that adaptation's votes.
- The genre filter and the period work as for the other metrics. A vote counts by the time its value was set or last changed.
- The snapshot stores, for each book, the source adaptation with its vote count and raw share. The public and admin collection items get an `adaptation` object, and `/top/{slug}` names the adaptation under each place.
- Admin: the two metrics in «Показатель», with «Минимум голосов» and «Порог доверия (m)», and scores shown as percentages.
- Forward-only migration `000036`: the metric CHECK constraint is widened, and three nullable source columns are added to the snapshot items.
- Not in scope: the «Посмотреть после прочтения» shelf, the «Прочитай перед премьерой» challenge and a shareable verdict card.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `collections/curated-collections`: new metrics in «Automatic collection rule», a new «Verdict score» requirement, the source adaptation in «Automatic snapshot», `adaptation` on items of «Public collection API», and the caption and row line in «Public collection pages».
- `collections/curated-collection-admin`: the metric list and rule defaults in «Create, edit and delete a collection», `adaptation` on items in «Collection detail for admins», and the metric options, rule fields and score format in «Automatic collection admin page».

## Impact

- backend: curated module (domain ranking, repository aggregate and snapshot write, read-time source lookup, transport shapes, swagger); migration `000036_verdict_collections`. No new endpoint and no authorization change.
- frontend: `packages/api-client` types; `apps/web` caption and row line on `/top/{slug}`; `apps/admin` metric options, rule form, snapshot table.
- devops: none. The existing `AUTO_COLLECTIONS_SCHEDULE_*` worker schedule recomputes the new metrics.
- docs: `docs/admin-guide/collections.md`, `docs/ROADMAP.md` (#510 moves out of the queue and gets a done entry, as #509 did).
