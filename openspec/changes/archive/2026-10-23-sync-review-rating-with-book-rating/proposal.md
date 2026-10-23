# Proposal

Issues: #380, #381.

## Why

A reader's rating of a book and the rating shown on their review can disagree: re-rating with the stars leaves the review on the old value (#381), and deleting a review silently keeps the rating without telling the reader (#380). The rating becomes the single source of truth and the review always shows it; the reader is told the rating survives a review deletion.

## What Changes

- Setting a rating directly (`POST /books/{id}/ratings`) also updates the rating of the reader's review of that book (visible or hidden) in the same transaction. The response shape is unchanged.
- A one-time forward data migration aligns every existing review's rating with the reviewer's current rating.
- Deleting a review keeps the rating (unchanged behavior, now explicit); the delete confirmation reads «Удалить рецензию? Ваша оценка книги сохранится.»
- A rating cannot be removed (no endpoint, no "clear" action) — made explicit, so a review can never be left without a rating.
- Book page: after a star change the reader's own review shows the new rating without a reload; after saving a review the star widget shows «Моя оценка: N» with the review's rating and the rating summary refreshes.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `reviews/book-ratings`: direct rating also updates the reviewer's review; a rating cannot be removed; the widget and the own review stay in sync on the book page.
- `reviews/book-reviews`: the review's rating always equals the reviewer's current rating; the delete confirmation says the rating stays.

## Impact

- backend: ratings repository (sync write in the upsert transaction), one data-only migration, demo seed reconcile.
- frontend: apps/web — small shared "my rating" store (entities layer), rate-book feature, reviews-section widget.
- devops: none.
- docs: none beyond the specs (archived by the orchestrator).
