# Proposal

Fixes GitHub issue #687 «Harvest attaches wrong Open Library covers from loose title search».

## Why

The harvest takes the first Open Library title search result as the work's match, even when Open Library ignored the query and returned an unrelated book (for example «История Петра I» by Pushkin → "I, Robot" by Asimov). Harvested books get another book's cover, ISBN and description, and several local books already share a wrong cover.

## What Changes

- The harvest accepts an Open Library title search result only when its title matches the work's Russian or English Wikidata label and one of its authors matches one of the work's authors (by surname, Russian or English label). It checks up to 5 results and uses the first that matches. Without a match, nothing is taken from Open Library for that work.
- The harvest query also reads the English labels of the work and its authors. They are used only for matching, never stored.
- A title search answer that matches more than 10 000 books is treated as no match, because it means Open Library ignored the query. This applies to the harvest and to the admin book lookup (which then answers 404 instead of returning an unrelated book). The admin lookup does not get the strict title/author check: it is a prefill that the admin reviews.
- New maintenance command `olcoveraudit` (command line only, no HTTP endpoint). By default it only reports harvested books whose cover came from an Open Library title search and looks wrong: the cover is shared with another book, or a strict re-check finds no match or a different cover. With `-apply -ids=…` it clears the cover link and ISBN of exactly the listed books through the normal book save, so the stored cover copy follows `catalog/book-covers`.

## Capabilities

### New Capabilities

### Modified Capabilities

- `catalog-import/wikidata-book-harvest`: the Open Library title search match guard; the new audit/cleanup command for existing wrong links.
- `catalog-import/book-lookup`: the "Open Library ignored the query" guard returns 404.

## Impact

- backend: `pkg/bookdata` (Open Library matcher, search `numFound`, English labels in the author-list SPARQL query), harvest `enrichRecord`, new `cmd/olcoveraudit` with a service method and a repository query, `backend/Dockerfile` build/copy lines.
- frontend: none.
- devops: one new binary in the backend image. It is run by hand in the `worker` container; no new volume or service.
- docs: `docs/DEPLOYMENT.md` (how to run the audit), `docs/ROADMAP.md` entry.
