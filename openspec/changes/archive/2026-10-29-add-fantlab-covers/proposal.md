# Proposal

Issue: #686 (FantLab as a second cover source for Russian editions). The original proposal assumed an ISBN lookup. Owner decisions from 2026-10-29 replace that approach.

## Why

586 harvested books still have no cover, because Open Library has nothing usable for Russian editions. 365 of them have a FantLab work ID (Wikidata P7439). A Russian paper edition on FantLab whose name equals the work name gives a safe cover for about 25-30 of them. We have no permission to keep copies yet, so these covers are only linked (hotlinked), never copied.

## What Changes

- Harvest enrichment gets a FantLab step for the cover only. The order is: ISBN, then the Open Library ID cover, then the FantLab self-titled Russian paper edition, then the strict Open Library title search. The FantLab step uses the Wikidata FantLab work ID (P7439). It accepts only an edition whose normalized name equals both the book's Russian title and the FantLab work name, and that has an image. It never takes a collection under another name.
- FantLab requests are at least 1.2 s apart (robots.txt Crawl-delay). Each work makes at most 4 FantLab requests, and each harvest job makes at most 250.
- The existing cover backfill command (`olcoverbackfill`) also tries FantLab for candidates that the Open Library IDs did not cover. It still only reports by default and writes only with the apply option.
- Book covers get a "link-only" host list, `fantlab.ru` by default, which is configurable. A cover on such a host is never downloaded. Its copy gets the new state `linked` instead of `failed`, and the source link is shown. **BREAKING (minor, admin API):** `cover_copy.status` can now be `linked`.
- The admin edit page shows a hint for `linked` covers.
- The public site renders link-only covers unoptimized, so the visitor's browser loads them straight from fantlab.ru. The Next image optimizer does not fetch or cache them, and `fantlab.ru` is deliberately left out of `images.remotePatterns`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `catalog-import/wikidata-book-harvest`: the FantLab cover step in Open Library enrichment and its order; the cover backfill also uses FantLab.
- `catalog/book-covers`: the `linked` copy state, the link-only hosts, the displayed link for linked covers, and copies queued for FantLab covers.
- `catalog-admin/book-editing`: `linked` in `cover_copy.status` and its hint on the edit page.

## Impact

- backend: new FantLab client in `pkg/bookdata` (P7439 in the harvest SPARQL plus a batched lookup); harvest handler option and worker wiring; backfill extension; covers module link-only handling; a config variable; a forward migration that adds `linked` to the `cover_status` CHECK; Swagger enum.
- frontend: `apps/web`: a shared helper that renders link-only covers unoptimized at every cover `<Image>`. `apps/admin`: the hint text and the `AdminCoverCopyStatus` type in `packages/api-client`.
- devops: `COVERS_LINK_ONLY_HOSTS` in both compose files.
- docs: `docs/DEPLOYMENT.md` (the env var and the backfill's FantLab step), plus the follow-up (attribution link to the FantLab edition page) noted in the ROADMAP.
