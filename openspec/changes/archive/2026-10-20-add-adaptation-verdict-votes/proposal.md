# Proposal

Issue: #509 (part 2 of 3 of «Книга или экранизация?», epic #512; builds on #508, #510 builds on it).

## Why

Readers who have read a book want to know, and say, whether the book or its screen adaptation is better. No other book catalog offers this per adaptation, and the «Экранизации» block already has the data.

## What Changes

- New per-reader vote for each (book, visible adaptation) pair: «Книга лучше» / «Экранизация лучше» / «Одинаково». The reader can change or remove it. Only readers who have the book on «Прочитано» can cast or change a vote. Removing is always allowed.
- New public endpoint with the community verdict per adaptation, plus the caller's own vote and whether they may vote. The vote counts are revealed only from 5 votes.
- New endpoints to set (`PUT`) and remove (`DELETE`) the caller's vote.
- The «Экранизации» block on the web book page shows under each adaptation the verdict («78% читателей: книга лучше · 23 голоса», or «Пока мало голосов» below 5 votes), the voting buttons for eligible readers, and a prompt for guests and for readers who have not read the book.
- Lifecycle: votes are kept when the book leaves «Прочитано» and when an admin hides the link (they are not shown). Deleting the link removes its votes, and so does deleting the book or the user.
- Owner decisions: no «с чего начать» advice. No verdict on the author page («Фильмы об авторе»), because biopics and documentaries are not adaptations. No admin UI in v1.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `screen-works/book-adaptations`: adds the vote endpoints, the verdicts endpoint, the vote lifecycle rules, and the verdict and voting UI on the book page.

## Impact

- backend: migration `000035` (vote table), screenworks module (domain/repository/service/transport), router, swagger.
- frontend: web only (api-client and types, `entities/screen-work`, new feature `vote-adaptation`, book page view, shelf-toggle cache invalidation). Admin app untouched.
- devops: none (no new config, worker untouched).
- docs: swagger regeneration; the admin guide gets a note on what hide and delete do to votes.
