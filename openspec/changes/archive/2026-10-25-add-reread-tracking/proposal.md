# Proposal

Delivers the re-read tracking part of GitHub issue #204 ("Explicit start/finish reading dates and re-read tracking"). Editing start/finish dates by hand and a reading-history list stay future work (phase 2).

## Why

The reading status is only "which default shelf the book is on", so moving a book from «Прочитано» back to «Читаю» to re-read it deletes the only record of the read. The reader loses challenge credit for past years and the right to vote on adaptations. A reading journal next to the shelves keeps that history.

## What Changes

- New reading journal per user and book: open, finished and abandoned attempts. The shelf is the current status and the journal is the history. Every default-shelf add or remove updates the journal in the same transaction, following fixed transition rules.
- Removing a book from «Прочитано» now works as an undo. It deletes only the latest finished reading, not the whole history.
- «Брошено» (`abandoned`) becomes a fourth default shelf in the exclusive reading-status group. It is created lazily like the other three and is offered in the book-page status chooser. Moving a book there closes the open reading as abandoned. Book cards keep their three buttons.
- Book page: a «Читаю снова» button when the status is «Прочитано», a «перечитываю» mark on «Читаю» while re-reading, and the counter «Прочитано N раз · последний раз — <месяц год>».
- `GET /api/v1/shelves?book_id=` returns a `reads` summary for the caller. Shelf book items (own and public shelf endpoints) carry `reads_count` and `last_read_at`. Own and public shelf pages show «Прочитано N раз».
- **BREAKING (behavior)**: reading-challenge counting moves from shelf `added_at` to finished readings. A book counts once per calendar year in which it was finished, and moving it from «Прочитано» to «Читаю» no longer lowers the count.
- **BREAKING (behavior)**: adaptation voting (`can_vote`, PUT vote) requires at least one finished reading instead of the book being on «Прочитано» right now, so a re-reader keeps voting.
- A one-time backfill builds the journal from the current shelves, so existing counts and voting rights stay the same.
- ROADMAP note: #204 phase 2 (manual date editing, reading-history list).

## Capabilities

### New Capabilities

- `collections/reading-journal`: journal model, status-transition rules, undo, abandoned handling, per-book reading summary endpoint, backfill.

### Modified Capabilities

- `collections/shelves`: «Брошено» becomes a default shelf; the default shelf order includes it.
- `collections/shelf-books`: the exclusive group is four shelves; the book-page status control gets «Брошено», «Читаю снова», «перечитываю» and the read counter; shelf book items carry read counts.
- `collections/public-collections`: the public shelf page shows the owner's read counter per book.
- `reading-challenges/progress`: books read in a year are counted from finished readings.
- `screen-works/book-adaptations`: vote eligibility and `can_vote` are based on at least one finished reading.

## Impact

- backend: new migration `book_reads` with backfill; shelves module (journal domain, repository, transitions in AddBook/RemoveBook, abandoned in default shelves, read counts in responses); reading_challenges repository; screenworks `IsReader`.
- frontend: apps/web shelve-book feature (status chooser, «Читаю снова», mark, counter), shelf type labels in shelves / public-profile / public-shelf-detail views, read counter on shelf pages; packages/api-client types.
- devops: none. The migration runs with the normal migrate step.
- docs: ROADMAP (#204 split into delivered part and phase 2), swagger annotations.
