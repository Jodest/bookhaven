# Proposal

Issue: #688

## Why

All 659 local books without a cover come from Wikidata and have no ISBN, and the strict title search finds almost none of them. 85 of them have an Open Library ID in Wikidata (P648), and 74 of those works have a cover in Open Library, so an exact lookup by ID can give them a cover.

## What Changes

- The harvest SPARQL query also reads the work's Open Library IDs (P648).
- Open Library enrichment gets a new step for the cover only. When the book still has no cover after the ISBN lookup, the job reads the work or edition record behind those IDs and takes its first positive cover ID. This happens before the strict title search. The order of the IDs is fixed and at most 3 are tried. The cover is used even when it belongs to an edition in another language (owner's decision: no marker).
- No description, ISBN or page count is taken from these records. The title search keeps filling those fields as it does today.
- New maintenance command for a one-off backfill. It finds Wikidata books without a cover link, reads their Open Library IDs from Wikidata in batches and applies the same rule. By default it only reports. With an explicit apply option it sets the cover link through the normal book save, so a cover copy is queued.
- Admin book lookup, file import and the create-only rule of the harvest do not change. Existing books get covers only through the backfill command.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `catalog-import/wikidata-book-harvest`: Open Library enrichment gets the cover-by-ID step (MODIFIED). New requirement for the cover backfill command (ADDED).
- `catalog/book-covers`: queuing a copy now also covers a cover filled in by Open Library ID during a harvest or by the backfill command (MODIFIED).

## Impact

- backend: `pkg/bookdata` (SPARQL field, Open Library work/edition cover lookup, Wikidata batch lookup of P648 by QID), harvest enrichment, new admin repository query, new `cmd/olcoverbackfill`.
- frontend: none.
- devops: `backend/Dockerfile` builds and ships the new command binary in the worker image.
- docs: `docs/DEPLOYMENT.md` (how to run the backfill), `docs/ROADMAP.md`.
