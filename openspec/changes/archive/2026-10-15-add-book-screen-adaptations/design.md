## Context

F1 of a series. F2 will add films about an author (biopics and documentaries, linked by P921 "main subject" or similar). F3 will add manual creation and editing of works and links in the admin. F1 must lay a model that both extend with additive migrations only.

Live check on 142 local books: P144 ("based on") finds 30 works for 17 books. Coverage: a Russian label for 29 of 30, P577 for 27, P345 for 30, P2603 for 28, P57 for 28. Traps:
- P144 also returns games, board games, list articles («film and television adaptations of The Hobbit») and film-series items, which duplicate their films.
- Several P577 values multiply SPARQL rows.
- Poster images are not freely licensed.

## Decisions

1. **New `screen-works` OpenSpec domain and a new backend module `screenworks`.** It owns the tables, the public endpoint, the admin link endpoints and the sync service. The sync capability lives in `catalog-import`, next to author enrichment, because it is a Wikidata import. F2 adds `screen-works/author-films`, and F3 adds an editing capability.
2. **Work and link are separate rows.** `screen_works` is a work, and `book_screen_works` links it to one book. A film based on two books is one work with two links. F2 adds `author_screen_works(author_id, screen_work_id, relation, hidden, source, …)` with the same shape. Hiding is per link, because a work can be right for one book and wrong for another.
3. **Hide, never delete.** A link carries `hidden`. The sync inserts links with `ON CONFLICT DO NOTHING`, so a hidden link is never unhidden or recreated. The sync never deletes works or links, even when Wikidata drops them.
4. **Fill empty fields only, and never touch a work edited by hand.** This mirrors author enrichment. For an existing work:
   - `title` and `kind` never change;
   - `year`, `original_title`, `director`, `imdb_id` and `kinopoisk_id` are filled only when NULL, in one `UPDATE … SET col = COALESCE(col, $new)`, so a concurrent manual value wins;
   - a work with `manually_edited_at IS NOT NULL` is skipped. The column is added now, with nothing setting it in F1. F3 sets it on every admin edit, so the rule "sync never overwrites manual edits" is in place from day one.
5. **Immediate publication, with no status on works or links.** Visibility is `link.hidden = false AND book.status = 'published'`. The human decided this (no moderation queue). Hide and show is the escape hatch, so the minimal hide UI is part of F1 and not deferred to F3. It is also what makes "hidden links do not return" verifiable end to end.
6. **Separate public endpoint `GET /books/{id}/adaptations`**, not a field in `GET /books/{slug}`. This is the same reasoning as `GET /books/{id}/collections` (archived change `add-collection-badges-home`, Decision 1): the `catalog` module does not query another module's tables, and other consumers of the book payload do not pay for it.
7. **One SPARQL query per chunk of up to 50 book QIDs.** The alternative is SPARQL for the pairs plus `wbgetentities` for the works and directors. SPARQL grouping removes the duplicate rows from P577 and P57 in one call. An admin request of 50 books is therefore exactly one Wikidata call, and a harvest of 500 created books is 10 calls. All calls go through the shared rate-limited `WikidataClient`.
8. **Kind is a closed set guarded by a CHECK constraint** (`film`, `tv_film`, `series`, `animated_film`, `animated_series`, `teleplay`). If F2 needs `documentary`, it is a one-line additive `ALTER … CHECK`. The book-or-author relation never lives in `kind`.
9. **Sync is allowed for books of any status**, as author enrichment is. Pending harvested books get their links early, and those links become public on publication.
10. **The harvest syncs only the books it created**, after author enrichment. Books it skipped are left alone, because harvesting never edits existing books.

## API

### `POST /api/v1/admin/books/sync-adaptations` (admin, bearer)
- Request: `{"ids": ["<uuid>", …]}`, 1 to 50 after dedupe.
- Response 200:
  ```json
  {"processed": 2, "updated": 1, "added": 3,
   "items": [{"book_id": "…", "title": "Мастер и Маргарита", "outcome": "added", "found": 3, "added": 3, "notes": [], "message": ""}]}
  ```
- `outcome` is `added | unchanged | skipped | error`. `notes` may contain `no_qid` and `wikidata_unavailable`.
- 400: `ids must not be empty`, `too many book ids: at most 50 per request`, or a malformed id. 503: `adaptation sync source is not configured`. 502: every Wikidata call failed, and the report is discarded as in author enrichment.
- Register the route in the admin group before any `/admin/books/:id` POST routes. Check for conflicts; there is no POST `/admin/books/:id` today.

### `GET /api/v1/books/{id}/adaptations` (public)
- Response 200: `{"items": [{"id", "title", "original_title", "kind", "year", "director", "imdb_url", "kinopoisk_url"}], "total": n}`.
- 400 `{"error":"invalid book id"}`, 404 `{"error":"book not found"}` (unknown or not published), 500 `{"error":"failed to get book adaptations"}`.
- The route goes next to `/books/:id/collections` and keeps the param name `:id`.

### `GET /api/v1/admin/books/{id}/adaptations` (admin)
- Same items plus `hidden`, `source`, `qid`, `imdb_id`, `kinopoisk_id`, and the same order.
- 404 for an unknown book.

### `PATCH /api/v1/admin/books/{id}/adaptations/{screen_work_id}` (admin, bearer)
- Request: `{"hidden": bool}`. The field is required: decode into `*bool`, and nil returns 400.
- 200 returns the admin item. 404 `{"error":"adaptation link not found"}` for an unknown book or link.

### Admin book responses
- Add `external_id` to the admin book DTO in `admin/transport/http/handler.go`. Today only the moderation item carries it. Add it to `AdminBook` in `packages/api-client`, so the list and the edit page can tell whether a book has a QID: `external_source === 'wikidata' && external_id`.

### Harvest result
- Add `adaptations_added` and `adaptation_sync_errors` to `HarvestResult` (`*int`, `omitempty`), the same pattern as `authors_enriched`. The jobs page is not changed in F1.

Swagger: Russian annotations for every new handler (`@Tags screen-works`, and `admin` for the admin ones), then regenerate `backend/docs` the usual way.

## Database

Migration `000029_screen_works` (up and down). It is forward-only and edits no existing migration.

```sql
CREATE TABLE screen_works (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title              VARCHAR(500) NOT NULL,
    original_title     VARCHAR(500),
    kind               VARCHAR(30) NOT NULL CHECK (kind IN ('film','tv_film','series','animated_film','animated_series','teleplay')),
    year               INTEGER CHECK (year BETWEEN 1880 AND 2100),
    director           VARCHAR(500),
    imdb_id            VARCHAR(20) CHECK (imdb_id ~ '^tt[0-9]+$'),
    kinopoisk_id       VARCHAR(20) CHECK (kinopoisk_id ~ '^[0-9]+$'),
    external_source    VARCHAR(50),
    external_id        VARCHAR(255),
    manually_edited_at TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK ((external_source IS NULL) = (external_id IS NULL))
);
CREATE UNIQUE INDEX uq_screen_works_external ON screen_works (external_source, external_id) WHERE external_id IS NOT NULL;

CREATE TABLE book_screen_works (
    book_id        UUID NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    screen_work_id UUID NOT NULL REFERENCES screen_works(id) ON DELETE CASCADE,
    hidden         BOOLEAN NOT NULL DEFAULT FALSE,
    source         VARCHAR(20) NOT NULL DEFAULT 'wikidata' CHECK (source IN ('wikidata','manual')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (book_id, screen_work_id)
);
CREATE INDEX idx_book_screen_works_work ON book_screen_works (screen_work_id);
```

Down: `DROP TABLE book_screen_works; DROP TABLE screen_works;`. It is never run by agents.

The work upsert is `INSERT … ON CONFLICT (external_source, external_id) WHERE external_id IS NOT NULL DO NOTHING RETURNING id`, followed by a select when nothing is returned, then the fill-empty update from Decision 4. The link insert is `INSERT … ON CONFLICT (book_id, screen_work_id) DO NOTHING`, and `added` counts the inserted rows. Run each book in one transaction.

F2 and F3 are additive on top of this:
- F2: a new `author_screen_works` table, and possibly a widened `kind` CHECK.
- F3: manual works with NULL `external_*`, `source='manual'` links, and `manually_edited_at` set by edits.

## Wikidata query (bookdata)

Add a new method to `WikidataClient` in a new file `bookdata/wikidata_adaptations.go`:
`FetchAdaptations(ctx, bookQIDs []string) (map[string][]ExternalScreenWork, error)`.
- 1 to 50 QIDs, each matching `^Q[0-9]+$`, otherwise `ErrInvalidQuery` without a call.
- The result is keyed by book QID.
- `ExternalScreenWork{QID, Title, OriginalTitle, Kind, Year *int, Director, IMDbID, KinopoiskID}`.
- It runs through `sparqlBindings`, which brings the rate limit and retries.

Shape of the query. The developer finalises it, and grouping is mandatory:
```sparql
SELECT ?book ?work
       (GROUP_CONCAT(DISTINCT STRAFTER(STR(?cls), "entity/"); separator="|") AS ?classes)
       (SAMPLE(?ru) AS ?ruLabel) (SAMPLE(?en) AS ?enLabel) (SAMPLE(?orig) AS ?origTitle)
       (MIN(YEAR(?date)) AS ?year)
       (GROUP_CONCAT(DISTINCT ?imdb; separator="|") AS ?imdbIds)
       (GROUP_CONCAT(DISTINCT ?kp; separator="|") AS ?kpIds)
       (GROUP_CONCAT(DISTINCT COALESCE(?dirRu, ?dirEn); separator="|") AS ?directors)
WHERE {
  VALUES ?book { wd:Q… }
  ?work wdt:P144 ?book ; wdt:P31 ?cls .
  OPTIONAL { ?work rdfs:label ?ru FILTER(LANG(?ru) = "ru") }
  OPTIONAL { ?work rdfs:label ?en FILTER(LANG(?en) = "en") }
  OPTIONAL { ?work wdt:P1476 ?orig }
  OPTIONAL { ?work wdt:P577 ?date }
  OPTIONAL { ?work wdt:P345 ?imdb }
  OPTIONAL { ?work wdt:P2603 ?kp }
  OPTIONAL { ?work wdt:P57 ?dir .
             OPTIONAL { ?dir rdfs:label ?dirRu FILTER(LANG(?dirRu) = "ru") }
             OPTIONAL { ?dir rdfs:label ?dirEn FILTER(LANG(?dirEn) = "en") } }
}
GROUP BY ?book ?work
```
Class filtering, kind mapping, id validation, the director cap and sorting are done in Go on `?classes` by a pure function with unit tests, and not in SPARQL. This keeps the whitelist testable and the query simple.

**Class lists.** Every QID here MUST be verified against Wikidata (label and description) before hardcoding, and the verified list is recorded in a code comment.

| kind (priority order) | allowed P31 classes (candidates) |
|---|---|
| `teleplay` | the item whose Russian label is «телеспектакль» or "television play". Look it up. Do NOT use `Q21191270` for this: it is believed to be "television series episode". |
| `animated_series` | Q581714 animated series; Q117467246 animated television series |
| `series` | Q5398426 television series; Q1259759 miniseries |
| `animated_film` | Q202866 animated film; Q29168811 animated feature film |
| `tv_film` | Q506240 television film |
| `film` | Q11424 film; Q24869 feature film; Q24862 short film |

Exclusion list. Any match drops the item, even when it also has an allowed class:
- Q24856 film series;
- Q7889 video game;
- Q131436 board game;
- Q13406463 Wikimedia list article;
- Q21191270 television series episode;
- Q7725310 series of creative works.

Title, year, director and id rules are as in the spec. `original_title` is stored only when it differs from the chosen title. An empty title drops the item.

## Backend layout

- `modules/screenworks/domain`: `ScreenWork`, `BookScreenWork`, the kind and source constants, and the kind priority.
- `modules/screenworks/repository` (GORM):
  - `UpsertWikidataWork`;
  - `LinkBook` (returns inserted bool);
  - `ListPublicByBook`, with a join on `books.status='published'` and `hidden=false`;
  - `ListAdminByBook`;
  - `SetLinkHidden`;
  - `GetBooksForSync(ids)`, which reads `books.id, title, status, external_source, external_id` (read-only).
- `modules/screenworks/service`:
  - `SyncBooks(ctx, ids)` is the admin entry. It validates 1 to 50 and returns `ErrSyncUnavailable`, `ErrSyncEmpty` or `ErrSyncTooLarge`.
  - `SyncCreatedBooks(ctx, ids)` is for the harvest. It has no cap and works in chunks of 50.
  - `HasSyncSource()`.
  - Public list, admin list and set hidden.
  - The "every chunk failed" handling is the same as `enrichAuthors` in `admin/service/author_enrich.go`.
- `modules/screenworks/transport/http`: the public handler and the admin handlers.
- Harvest:
  - `processHarvestRecord` must also return the created book id.
  - `harvest/service/handler.go` collects the created book ids and, after `enrichCreatedAuthors`, calls an injected `AdaptationSyncer` interface (`SyncCreatedBooks`, `HasSyncSource`), declared in the harvest package so that it does not import screenworks internals.
  - A nil syncer skips the step and leaves both counters nil.
- Wiring: `cmd/api` and `cmd/worker` pass the same `*bookdata.WikidataClient` they already share for enrichment.

## Frontend

- `packages/api-client`:
  - types `ScreenWorkKind`, `BookAdaptation`, `AdminBookAdaptation`, `AdminAdaptationSyncReport` and its item;
  - `books.adaptations(id)` (public);
  - `admin.books.syncAdaptations(ids)`, `admin.books.adaptations(id)` and `admin.books.setAdaptationHidden(bookId, workId, hidden)`;
  - `external_id` on `AdminBook`.
- `apps/web`:
  - New entity `entities/screen-work`: `screenWorksApi.forBook(id)`, the kind label map (`film` «Фильм», `tv_film` «Телефильм», `series` «Сериал», `animated_film` «Мультфильм», `animated_series` «Мультсериал», `teleplay` «Телеспектакль»), and the `BookAdaptations` UI. The UI renders nothing for an empty list. Links use `target="_blank" rel="noopener noreferrer"`.
  - `views/book-detail` fetches it like `collectionsApi.forBook(book.id).catch(() => null)` and renders the block after the description. Pick the exact position next to the existing sections, before reviews.
- `apps/admin`:
  - New feature `features/sync-adaptations`: a `useSyncAdaptations` mutation and `lib/sync-text.ts` with `syncItemText`, `bulkSyncSummary` and `syncErrorMessage`, mirroring `features/enrich-author/lib/enrich-text.ts`. The mutation invalidates `['admin','books']` and the book's adaptations query.
  - `views/books-list`: a `renderRowActions` button for QID books, as `views/authors` does.
  - `views/book-moderation`: a row action and a bulk action counting QID rows, as `views/author-moderation` does.
  - `views/book-edit`: the «Экранизации» section, a widget `widgets/book-adaptations-admin` or a block inside the view. It uses `entities/book` for the admin adaptations API. The view composes the feature and the widget, and there are no cross-feature imports.
- Strings as quoted in the specs.

## Existing Patterns

- Admin batch action, its report, the «every chunk failed means 502» rule and 503 when not configured: `backend/internal/modules/admin/service/author_enrich.go` and `handler_author_enrich_test.go`.
- Post-job best-effort step and its counters: `harvest/service/handler.go` (`enrichCreatedAuthors`, `AuthorsEnriched`) and `handler_enrich_test.go`.
- SPARQL execution: `bookdata/wikidata.go` (`sparqlBindings`, `escapeSPARQLString`, `extractWikidataID`).
- Public per-book endpoint: `curated/transport/http/handler.go` `ListBookPlaces`, its route next to `/books/:id/similar`, and the router test proving that anonymous access is not 401 or 403.
- Banner texts and bulk with the QID count: `apps/admin/src/features/enrich-author`, `views/authors`, `views/author-moderation`.
- Migration style: `000025_curated_collections.up.sql`.

## Risks

- **Class QIDs.** A wrong whitelist QID either leaks games and lists or misses teleplays. Mitigations: verify every QID (task 1.2), apply the exclusion list after the whitelist, and keep hide as the fallback.
- **Immediate publication of bad data.** A mislabeled Wikidata item goes public at once on published books. Mitigation: hide in the admin, which is permanent across syncs.
- **SPARQL timeouts on 50 QIDs.** The P57 label sub-joins add cost. If the endpoint times out, lower the chunk to 25 internally and keep the API cap at 50.
- **Book QID redirects.** P144 points to the canonical QID, so a book stored under a redirected QID finds nothing. Accept this and do not resolve redirects in F1.
- **Kinopoisk links for series.** `/film/<id>/` redirects to `/series/<id>/` on Kinopoisk. Accept this.
- **Harvest duration.** A 5000-work harvest adds up to 100 SPARQL calls, which is still within the 30-minute timeout at the current rate limit. Monitor it.
- **New domain `screen-works`.** The domain lists in `openspec/config.yaml`, `docs/PIPELINE.md` and the feature-planner agent prompt need the new entry. The orchestrator decides on the agent prompt.
