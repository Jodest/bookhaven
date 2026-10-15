## Context

F2 of the screen works series. F1 (`openspec/changes/archive/2026-10-15-add-book-screen-adaptations/`) built `screen_works` + `book_screen_works`, the `screenworks` module, `bookdata.FetchAdaptations` and the harvest `AdaptationSyncer` step, and planned F2 as "a new `author_screen_works` table with the same shape". This change follows F1 almost one to one, so the developer should copy F1's code paths and diverge only where noted. F3 (manual creation and editing of works and links) is out of scope.

Wikidata shape (checked by the requester): film `—P921 (main subject)→` author, with P31 in the video whitelist. Examples: Tolkien Q892 → «Толкин» (2019, Q42689885); Rowling Q34660 → «Magic Beyond Words» (2011, Q1343175, a TV film biopic).

## Decisions

1. **Same module, new capability.** Author links live in the `screenworks` module next to book links (domain, repository, service, transport). Spec capabilities: `screen-works/author-films` (public + admin links) and `catalog-import/wikidata-author-film-sync` (sync), mirroring F1's split.
2. **`relation` on the link, not a new `kind`.** `author_screen_works.relation IN ('biopic','documentary')`. `kind` stays the format (film, TV film, series...), as F1 Decision 8 required ("the book-or-author relation never lives in `kind`"). The `screen_works.kind` CHECK is **not** widened: a documentary is still a `film`/`tv_film`/`series`. This keeps migration 000030 purely additive (one new table) and keeps F1's book page and kind labels untouched. Trade-off: "documentary" is really a property of the work, so if F3 wants it on the work, it can add a column then; for F2 only author links need it.
3. **Filter: any allowed video work with P921 = author, no genre requirement.** Alternatives considered:
   - *Require genre biographical (Q645928) or documentary (Q93204).* Rejected: P136 coverage on Wikidata is patchy, especially for Russian/Soviet TV films and documentaries; it would drop true films about the author.
   - *Any item with P921 = author.* Rejected: P921 is used massively by books, scholarly articles, paintings, etc.

   P921 means "main subject" (not "depicts" P180, not "characters" P674), so a video work with the author as main subject is a film about the author even when it has several main subjects (a documentary about the Inklings is still about Tolkien). The class whitelist plus F1's exclusion list removes episodes of documentary cycles, film series and list articles, which are the noisy cases. Remaining false positives are handled by hide, which is permanent across syncs. Relation: `documentary` when a P31 class or a P136 genre is in the documentary set, else `biopic`.
4. **Class filter partly in SPARQL.** Unlike F1 (P144 targets are few), P921 on a famous author can match thousands of non-film items. The query MUST restrict `?work` to the allowed classes inside SPARQL (`FILTER EXISTS { ?work wdt:P31 ?a . VALUES ?a { … } }`), then collect all P31 classes and P136 genres with `GROUP_CONCAT` so Go can apply exclusions, kind priority and relation. The `VALUES` list is generated from the Go whitelist so there is one source of truth.
5. **Shared works.** Works are upserted by QID with F1's `upsertWikidataWork` (fill-empty-only, skip `manually_edited_at`, title and kind never change). A work first found as an adaptation is reused; its kind stays whatever was stored first.
6. **Relation is set on insert only.** Links use `INSERT … ON CONFLICT (author_id, screen_work_id) DO NOTHING`, so a hidden link is never unhidden, a re-sync never changes relation, and `added` counts only inserted rows.
7. **Public endpoint by author id.** The author page is routed by slug (`/authors/[slug]`) and already loads `authorsApi.get(slug)`, which returns `id`. The block fetches `GET /authors/:id/screen-works` with that id, exactly like F1's book page (`/books/:slug` + `/books/:id/adaptations`). The route goes next to `/authors/:slug`; Echo already resolves `/books/:slug` alongside `/books/:id/...`, and a router test proves it for authors too. A non-UUID returns 400.
8. **Visibility** = `link.hidden = false AND authors.status = 'published'`. Sync is allowed for authors of any status.
9. **Auto sync triggers.**
   - Direct import: only for outcome `created`, after `enrichImportedAuthor`, synchronous within the request (one SPARQL call for one QID), errors logged and swallowed. The import response is unchanged (no new field), so the existing `wikidata-author-import` requirements are not modified. AI curation imports go through the same endpoint, so they are covered.
   - Harvest: for `createdAuthors` (the set already passed to `enrichCreatedAuthors`), after the F1 adaptation step, via a new injected interface. Authors that a harvest merely matched, and scheduled harvests of existing flagged authors, are not synced; the admin buttons cover them.
   - Manual enrichment («Обогатить из Wikidata») does not trigger the film sync.
10. **Author merge keeps hides.** The merge deletes the duplicate, and the FK cascade would drop its links, including hides. The merge transaction moves links to the canonical author first, with `hidden = existing.hidden OR moved.hidden`. Without this, re-syncing the canonical author (which inherits the QID) would resurrect a film an admin hid.

## API

### `POST /api/v1/admin/authors/sync-screen-works` (admin, bearer)
- Request: `{"ids": ["<uuid>", …]}`, 1 to 50 after dedupe.
- Response 200:
  ```json
  {"processed": 2, "updated": 1, "added": 2,
   "items": [{"author_id": "…", "name": "Джон Р. Р. Толкин", "outcome": "added", "found": 2, "added": 2, "notes": [], "message": ""}]}
  ```
- `outcome`: `added | unchanged | skipped | error`; `notes`: `no_qid`, `wikidata_unavailable`; error messages `author not found`, `wikidata unavailable`, `failed to save author films`.
- 400: `invalid request`, `ids must not be empty`, `too many author ids: at most 50 per request`, `invalid author id: "<raw>"`. 503: `author film sync source is not configured`. 502: `wikidata is unavailable; retry later` (report discarded, as in F1).
- Register in the `authors` admin group **before** `authors.GET("/:id")` (next to `/enrich-wikidata`). No conflict with `POST /:id/merge`.

### `GET /api/v1/authors/{id}/screen-works` (public)
- 200 `{"items": [{"id", "title", "original_title", "kind", "relation", "year", "director", "imdb_url", "kinopoisk_url"}], "total": n}`.
- 400 `{"error":"invalid author id"}`, 404 `{"error":"author not found"}` (unknown or not published), 500 `{"error":"failed to get author films"}`.
- Route next to `api.GET("/authors/:slug", …)` in `transport/http/router.go`; same order as F1 (year asc, nulls last, title, id).

### `GET /api/v1/admin/authors/{id}/screen-works` (admin)
- Same items plus `hidden`, `source`, `qid`, `imdb_id`, `kinopoisk_id`. 400 invalid id, 404 `author not found`.

### `PATCH /api/v1/admin/authors/{id}/screen-works/{screen_work_id}` (admin, bearer)
- `{"hidden": bool}` decoded into `*bool`; nil → 400 `hidden is required`. 400 `invalid author id` / `invalid screen work id`. 404 `author film link not found` (unknown author or link). 200 admin item.

### Harvest result
- `HarvestResult` gains `AuthorFilmsAdded *int json:"author_films_added,omitempty"` and `AuthorFilmSyncErrors *int json:"author_film_sync_errors,omitempty"`, set only when the syncer is configured (0 when no author was created). The jobs page is not changed.

### Admin author DTO
- Already carries `external_id` (used by enrichment); no change. QID check on the frontend: `!!author.external_id` as in `views/authors`.

Swagger: Russian annotations (`@Tags screen-works` for public, `admin` for admin), regenerate `backend/docs` the usual way.

## Database

Migration `000030_author_screen_works` (up and down), forward-only, edits no existing migration, no change to `screen_works`.

```sql
CREATE TABLE author_screen_works (
    author_id      UUID NOT NULL REFERENCES authors(id) ON DELETE CASCADE,
    screen_work_id UUID NOT NULL REFERENCES screen_works(id) ON DELETE CASCADE,
    relation       VARCHAR(20) NOT NULL CHECK (relation IN ('biopic','documentary')),
    hidden         BOOLEAN NOT NULL DEFAULT FALSE,
    source         VARCHAR(20) NOT NULL DEFAULT 'wikidata' CHECK (source IN ('wikidata','manual')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (author_id, screen_work_id)
);
CREATE INDEX idx_author_screen_works_work ON author_screen_works (screen_work_id);
```

Down: `DROP TABLE author_screen_works;` — never run by agents.

Link insert: `INSERT … (author_id, screen_work_id, relation, hidden, source, …) VALUES (…, FALSE, 'wikidata', …) ON CONFLICT (author_id, screen_work_id) DO NOTHING`; `added += RowsAffected`. One transaction per author, reusing `upsertWikidataWork`.

Merge (admin repository `MergeAuthors`, inside the existing transaction, **before** `DELETE FROM authors`):
```sql
INSERT INTO author_screen_works (author_id, screen_work_id, relation, hidden, source, created_at, updated_at)
SELECT ?canonical, screen_work_id, relation, hidden, source, created_at, NOW()
FROM author_screen_works WHERE author_id = ?duplicate
ON CONFLICT (author_id, screen_work_id)
DO UPDATE SET hidden = author_screen_works.hidden OR EXCLUDED.hidden, updated_at = NOW();
DELETE FROM author_screen_works WHERE author_id = ?duplicate;
```
(The explicit delete keeps the "do not rely on cascade" style of `MergeAuthors`.) This is the only place the admin module writes the table; it is plain SQL in the same transaction, acceptable as the merge already owns cross-table consistency for `book_authors`.

## Wikidata query (bookdata)

New file `bookdata/wikidata_author_films.go`:
- `MaxAuthorFilmAuthors = MaxEntitiesPerRequest`.
- `type ExternalAuthorFilm struct { ExternalScreenWork; Relation string }` with `AuthorFilmRelationBiopic = "biopic"`, `AuthorFilmRelationDocumentary = "documentary"`.
- `FetchAuthorFilms(ctx, authorQIDs []string) (map[string][]ExternalAuthorFilm, error)`: 1..50 QIDs matching `^Q[0-9]+$` else `ErrInvalidQuery` without a call; result keyed by author QID; via `sparqlBindings`.
- Pure `classifyAuthorFilm(classes, genres []string) (kind, relation string, ok bool)`:
  1. any class in `screenExcludedClasses` → not ok (reuse F1's set);
  2. kind = F1 `screenKindPriority`, extended (for author films only, do not change `classifyAdaptation`) with lower-priority fallbacks: documentary TV series → `series`; television documentary → `tv_film`; documentary film Q93204, biographical film Q645928 → `film`;
  3. relation = `documentary` if any class or genre is in the documentary set (Q93204 plus the verified documentary TV classes), else `biopic`.
- Reuse F1 helpers `firstNonEmpty`, `parseScreenYear`, `joinDirectors`, `firstMatching` and the original-title rule.

**QIDs to verify live before hardcoding** (label + description, record in a code comment, like F1): Q93204 documentary film (expected used both as P31 and as P136), Q645928 biographical film, and look up the items for "television documentary" and "documentary television series" (do not guess; drop a class if it cannot be verified). Also confirm on Q42689885 and Q1343175 that the query returns them with the expected kind/relation, and on one known documentary that it is P31 Q93204 and/or P136 Q93204.

Query shape (developer finalises; class filter in SPARQL is mandatory, grouping is mandatory):
```sparql
SELECT ?author ?work
       (GROUP_CONCAT(DISTINCT STRAFTER(STR(?cls), "entity/"); separator="|") AS ?classes)
       (GROUP_CONCAT(DISTINCT STRAFTER(STR(?genre), "entity/"); separator="|") AS ?genres)
       (SAMPLE(?ru) AS ?ruLabel) (SAMPLE(?en) AS ?enLabel) (SAMPLE(?orig) AS ?origTitle)
       (MIN(YEAR(?date)) AS ?year)
       (GROUP_CONCAT(DISTINCT ?imdb; separator="|") AS ?imdbIds)
       (GROUP_CONCAT(DISTINCT ?kp; separator="|") AS ?kpIds)
       (GROUP_CONCAT(DISTINCT COALESCE(?dirRu, ?dirEn); separator="|") AS ?directors)
WHERE {
  VALUES ?author { wd:Q… }
  ?work wdt:P921 ?author .
  FILTER EXISTS { ?work wdt:P31 ?allowed . VALUES ?allowed { wd:Q11424 … } }
  ?work wdt:P31 ?cls .
  OPTIONAL { ?work wdt:P136 ?genre }
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
GROUP BY ?author ?work
```

## Backend layout

- `screenworks/domain`: `AuthorRef{ID, Name, Status, ExternalSource, ExternalID}` with `WikidataQID()`; `AuthorFilmInput{WorkInput; Relation}`; `RelationBiopic/RelationDocumentary`; `LinkedWork` gains `Relation string gorm:"column:relation"` (empty for book links) or a separate `LinkedAuthorFilm`; `ErrAuthorNotFound` («author not found»), `ErrAuthorLinkNotFound` («author film link not found»).
- `screenworks/repository`: `GetAuthorsForSync(ids)` (reads `authors.id, name, status, external_source, external_id`, read-only), `AuthorStatus(id)`, `SyncAuthor(authorID, []AuthorFilmInput) (int, error)`, `ListPublicByAuthor`, `ListAdminByAuthor`, `SetAuthorLinkHidden`. Reuse `upsertWikidataWork`, `linkedOrder`.
- `screenworks/service`: `AuthorSource` interface (`FetchAuthorFilms`), passed to `NewService` (the `*bookdata.WikidataClient` satisfies both; keep one constructor arg or add an option — developer's choice, keep `nil` = unavailable). `SyncAuthors(ctx, ids)` (1..50, errors `ErrAuthorSyncUnavailable`, `ErrSyncEmpty`, `ErrAuthorSyncTooLarge`), `SyncCreatedAuthors(ctx, ids) (added, failed int)`, `SyncImportedAuthor(ctx, id)` (or reuse `SyncCreatedAuthors` with one id), `HasAuthorSyncSource()`, `ListPublicByAuthor`, `ListAdminByAuthor`, `SetAuthorFilmHidden`. Prefer extracting the F1 chunk/outcome loop into a small generic helper over copy-paste, but do not change F1 behavior.
- `screenworks/transport/http`: `ListAuthorFilms`, `AdminListAuthorFilms`, `AdminSetAuthorFilmHidden`, `SyncAuthorFilms`.
- Admin import: `adminservice.WithAuthorFilmSyncer(AuthorFilmSyncer)` option; interface declared in the admin service package (`SyncCreatedAuthors(ctx, []uuid.UUID) (added, failed int)`; `HasAuthorSyncSource() bool`); called in `ImportWikidataAuthor` right after `enrichImportedAuthor` when outcome is `created`; result ignored except a log line.
- Harvest: `AuthorFilmSyncer` interface + `WithAuthorFilmSyncer` option in `harvest/service/handler.go`; after the adaptation step, if configured: counters set, `SyncCreatedAuthors(createdAuthors)` only when non-empty.
- Wiring: in `cmd/api/main.go` construct `screenWorksService` **before** `adminService` (it has no admin dependency) and pass it via `WithAuthorFilmSyncer`; `cmd/worker/main.go` adds `harvestservice.WithAuthorFilmSyncer(screenWorksService)` and, if the worker builds an admin service used for imports, the admin option too.
- Merge: SQL from Database inside `admin/repository.MergeAuthors`.

## Frontend

- `packages/api-client`: types `AuthorFilmRelation = 'biopic' | 'documentary'`, `AuthorFilm` (public item), `AdminAuthorFilm`, `AdminAuthorFilmSyncItem`, `AdminAuthorFilmSyncReport`; `authors.screenWorks(id)` (public); `admin.authors.syncScreenWorks(ids)`, `admin.authors.screenWorks(id)`, `admin.authors.setScreenWorkHidden(authorId, workId, hidden)`.
- `apps/web`:
  - `entities/screen-work`: `screenWorksApi.forAuthor(id)`, component `AuthorFilms` (heading «Фильмы об авторе», same item layout as `BookAdaptations`, reuse `screenWorkKindLabel`, «Документальный» as a small badge/text after the type for `relation === 'documentary'`; renders nothing for an empty list). Export from the entity index.
  - `views/author-detail`: `screenWorksApi.forAuthor(author.id).catch(() => null)` after the author load, render `<AuthorFilms>` after the «Книги автора» section (render even when the author has no books).
- `apps/admin`:
  - New feature `features/sync-author-films`: `useSyncAuthorFilms` mutation (invalidates `['admin','authors']` and the author's films query) and `lib/sync-text.ts` with `syncItemText`, `bulkSyncSummary`, `syncErrorMessage`, mirroring `features/sync-adaptations` with the author strings from the spec.
  - `views/authors`: a `renderRowActions` button for QID authors next to «Обогатить из Wikidata».
  - `views/author-moderation`: a row action and the bulk «Найти фильмы об авторе в Wikidata (N)» counting QID rows only, next to the enrich actions.
  - New widget `widgets/author-films-admin` (copy of `widgets/book-adaptations-admin`): uses `adminAuthorsApi` from `entities/author` for list/hide, composes `features/sync-author-films`; marks «Скрыт», «Документальный»; strings from the spec. `views/author-edit` renders it below the form with `hasQid = !!author.external_id`.
  - FSD: no cross-feature imports, no view imports a view.

## Existing Patterns

- Everything in F1: `screenworks/*`, `bookdata/wikidata_adaptations.go` (+ tests), `harvest/service/handler.go` (`AdaptationSyncer`, `WithAdaptationSyncer`) and its tests, `apps/web/src/entities/screen-work`, `apps/admin/src/features/sync-adaptations`, `apps/admin/src/widgets/book-adaptations-admin`.
- Author enrichment banners and QID bulk counting: `apps/admin/src/features/enrich-author`, `views/authors`, `views/author-moderation`.
- Import best-effort step: `admin/service/author_import.go` (`enrichImportedAuthor`) and `author_enrich_import_test.go`.
- Merge transaction: `admin/repository/repository.go` `MergeAuthors`.
- Migration style: `000029_screen_works.up.sql`.

## Risks

- **False positives from P921.** A fiction film with the author as one main subject among many is linked as a biopic. Mitigation: hide is permanent; exclusions remove episodes and series.
- **Class/genre QIDs.** A wrong documentary QID mislabels relation; a wrong whitelist QID leaks or misses works. Mitigation: verify every QID live (task 1.2), unit-test the classifier.
- **SPARQL cost.** `FILTER EXISTS` over P921 for 50 famous authors plus the genre join may be slow. If it times out, chunk 25 internally, keep the API cap at 50.
- **Relation frozen at insert.** If Wikidata later adds a documentary genre, an existing link stays `biopic`. Accepted; F3 editing can fix it.
- **Shared work kind.** A work first stored as an adaptation keeps its kind; acceptable since kind is the format.
- **Import latency.** A created import makes one extra SPARQL call within the request (bulk AI imports do this per author). Acceptable at the current rate limit; errors never fail the import.
- **Author QID redirects.** P921 points to the canonical QID; an author stored under a redirected QID finds nothing (same accepted limitation as F1).
- **Wiring order in `cmd/api`.** Moving `screenWorksService` construction above `adminService` must not change other wiring.
