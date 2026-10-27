# Design

## Context
Screen adaptations (`screen-works`) are stored in `screen_works` and linked to books through `book_screen_works` (migration 000029) and to authors (000030). The Wikidata sync code is:
- `backend/internal/pkg/bookdata/wikidata_adaptations.go`, the SPARQL query;
- `backend/internal/modules/screenworks/service/service.go` (`syncBooks`/`syncChunk`) and `catalog_sync.go`;
- `repository/repository.go` (`SyncBook`, `upsertWikidataWork`), which fills empty fields and skips works with `manually_edited_at`.

Images are handled in three places today:
- Book covers are mirrored by the worker into a content-addressed FS store (`internal/pkg/coverstore`, `COVERS_DIR`, volume `covers-data`). The api mounts that volume read-only and serves the files at `/covers/<shard>/<file>` (`modules/covers/transport/http`), and Next rewrites `/covers/*` to the api.
- Author photos are hotlinked Wikimedia Commons thumbnails with attribution columns (migration 000016). They are resolved by `WikidataClient.FetchCommonsImages` (`bookdata/commons.go`, free-licence allowlist) and rendered `unoptimized` in web.
- There is no admin image upload anywhere yet. The only multipart route is the book import, with a body-limit override in `internal/transport/http/limits.go`.

The book page view is `apps/web/src/views/book-detail/ui/book-detail-view.tsx`, a server component. It renders `BookAdaptations` (an entity, server-compatible), then `SimilarBooks` and `ReviewsSection` (client widgets). The vote UI is in `features/vote-adaptation`.

## Decisions

### D1. Verdict threshold 1, one constant
- Set `screenworks/domain.MinVerdictVotes = 1`; the handler already serialises it as `min_votes`. Restoring 5 later is a one-line edit.
- `curated/domain.DefaultMinVerdictVotes` stays **5**. The verdict collections, «книга лучше» and similar, need a real sample before a book enters a public collection. Replace the "keep equal" comments in both files with "intentionally independent".
- Web: `verdictLabel` returns `string | null`, and `null` below `min_votes` (also when the counts are `null`). `AdaptationVerdict` renders the `<p>` only for a non-null label. «Пока мало голосов» is deleted. The `border-t pt-3` wrapper class goes, and a small top margin is kept.

### D2. Vote dropdown
- Use the native `Select` from `@bookhaven/ui` (it is a `<select>` with an `options` prop), compact (`h-8 w-auto`). The visible label «Что лучше:» is a `<label htmlFor>` of the select.
- Option values:
  - `''` «Не выбрано», only when there is no vote;
  - `book`, `adaptation` and `same`, with the captions from `ADAPTATION_VOTE_SHORT_LABELS`;
  - `remove` «Убрать голос», only when there is a vote.
- The select is controlled by `item.my_vote ?? ''`. `onChange` maps `remove` to `setVote(null)` and any other value to `setVote(value)`. Because the value comes from the query data, a failed request automatically shows the previous value again. It is disabled while `isPending`.
- The «Ваш голос: … / Убрать голос» branch for users who may not vote is unchanged.

### D3. Book page tabs (SSR-safe)
- The page `app/books/[slug]/page.tsx` also receives `searchParams` and passes `tab` to `BookDetailView`. The view computes:
  - `hasAdaptations` = the adaptations loaded and there is at least one item;
  - `initialTab` = the requested tab if it exists, else `adaptations` when `hasAdaptations`, else `reviews`.
- New client component `views/book-detail/ui/book-tabs.tsx`. It lives in the same slice, so there is no cross-view import. It takes `tabs: {id, label, panel: ReactNode}[]` and `initialTab`. The panels are created by the server view, so `BookAdaptations` stays server-rendered HTML. Every panel is always rendered, and inactive ones get the `hidden` attribute.
  - ARIA:
    - `role="tablist"`, with `role="tab"` buttons carrying `aria-selected` and `aria-controls`;
    - a roving `tabIndex` (0 for the selected tab, -1 otherwise);
    - `role="tabpanel"` with `aria-labelledby` and `tabIndex={0}`;
    - Left/Right arrows wrap, and Home/End jump. Arrows select automatically.
  - The selected tab is `useSearchParams().get('tab')`, valid ids only, falling back to `initialTab`. Selecting calls `window.history.replaceState(null, '', url-with-tab)`. Next 15 syncs `useSearchParams` with native `replaceState` without an RSC round-trip and without scrolling.
  - On mount, `location.hash === '#reviews'` selects `reviews` (old `#reviews` links).
  - The tab list container has `id="book-tabs"` and `scroll-mt-24`.
- New client component `views/book-detail/ui/book-tab-link.tsx`, `<a href="?tab=<id>#book-tabs">`. On a plain left click it calls `preventDefault`, then `replaceState` with the tab, then scrolls `#book-tabs` into view, with `behavior: 'auto'` under `prefers-reduced-motion`. Without JS the href still works: the server renders that tab, and the browser jumps to the anchor. It is used for:
  - the header link «N экранизаций», rendered next to `RatingSummary` with the lucide `Clapperboard` icon, the accent colour and `pluralRu(n, ['экранизация','экранизации','экранизаций'])`;
  - the `RatingSummary` links that today use `href="#reviews"`, which become `BookTabLink tab="reviews"`. RatingSummary stays in the view slice.
- Tab labels: `Экранизации (${n})`, `Рецензии`, `Похожие книги`.
- The panels' own `h2` headings are visually hidden (`sr-only`), to avoid the tab and the heading repeating the same text:
  - `BookAdaptations` gets `headingHidden?: boolean`;
  - `SimilarBooks` and `ReviewsSection` get the same prop.

  The `id="reviews"` of `ReviewsSection` stays.
- `SimilarBooks` gets `emptyFallback?: ReactNode`. In the tab it receives `<p>Похожих книг пока нет.</p>`, which is rendered instead of `null` on an empty result or an error.
- SEO: `generateBookMetadata` sets `alternates.canonical` to `/books/<slug>`. Use `metadataBase` if the root layout has one, otherwise the site origin the sitemap already uses, so `?tab=` variants are not duplicates. Reading `searchParams` makes the page dynamic. That is acceptable, because it already fetches per request.

### D4. Poster storage: hotlink external, self-host uploads
- **Commons**: hotlink the 500 px thumbnail returned by `FetchCommonsImages`, with attribution columns. This is the exact author-photo pattern. Free licences allow it, and CC BY/BY-SA need the author and licence shown, which `PosterAttribution` does.
- **TMDB**: hotlink `https://image.tmdb.org/t/p/w342<poster_path>`. TMDB's terms expect use of their image CDN with attribution. Copying the images would add a worker pipeline and a licence question for no benefit.
- **Manual uploads**: stored on our server. They are needed because nothing else hosts them.
  - The covers store is not reused. Its orphan sweep deletes every file not referenced by `books`, and the api mounts it read-only.
  - Instead, a second `coverstore.NewFS(cfg.PostersDir)` is used: `POSTERS_DIR`, default `./storage/posters`, in a new volume `posters-data` mounted **read-write in the api only**. The worker never writes posters.
  - File names reuse the content-addressed scheme `{sha[0:2]}/{sha}.{ext}` (`coverstore.ValidName`), and the type is sniffed with `coverfetch.SniffExt`.
  - The files are served at `GET|HEAD /posters/:shard/:file`. Generalise `covers/transport/http.Handler` with `RegisterAt(e, prefix)`, keeping `Register` for `/covers`. The same immutable cache headers and nosniff apply.
  - Garbage collection is synchronous. After a replace, a removal or a work deletion, the previous file is deleted when no `screen_works.poster_path` references it any more.
- Web rendering:
  - `/posters/...` goes through `next/image` (optimised, local path, like `/covers`);
  - Commons and TMDB URLs are rendered `unoptimized`, like author photos, so `remotePatterns` does not change;
  - the CSP `img-src https:` already allows them.
  - Add the `/posters/:path*` rewrite and extend the headers exclusion to `(?!api/|covers/|posters/)` in **both** `apps/web/next.config.js` and `apps/admin/next.config.js`.

### D5. Poster state model
- `poster_source` is NULL (no poster, the sync may fill), `manual`, `commons`, `tmdb` or `removed` (an admin removed an automatic poster, and the sync never fills it).
- An upload sets `manual` from any state. A removal turns `manual` into NULL and `commons`/`tmdb` into `removed`. NULL and `removed` stay as they are.
- Poster writes **never** touch `manually_edited_at`. The sync fills only `WHERE poster_source IS NULL AND manually_edited_at IS NULL`. This keeps the existing rule «a work edited by hand is never changed by the sync», so the `Manual data wins over the sync` requirement is unchanged. Admins upload a poster by hand for such works.
- The API `poster` object is built in the domain: `LinkedWork.Poster()`, which returns `nil` for NULL and `removed`. For `manual` the `url` is `"/posters/" + poster_path`.

### D6. Sync integration
- SPARQL (`buildAdaptationsQuery`): add `OPTIONAL { ?work wdt:P3383 ?posterFile }`, `OPTIONAL { ?work wdt:P18 ?imageFile }`, `OPTIONAL { ?work wdt:P4947 ?tmdbMovie }` and `OPTIONAL { ?work wdt:P4983 ?tmdbTv }` with `SAMPLE` aggregates. P18/P3383 come back as `http://commons.wikimedia.org/wiki/Special:FilePath/<url-encoded name>`. A new helper `commonsFileName(raw)` takes the part after `Special:FilePath/`, path-unescapes it, and rejects an empty name or one containing `|`. The TMDB ids must match `^[0-9]{1,10}$`. `ExternalScreenWork` gains `PosterFile`, `ImageFile`, `TMDBMovieID` and `TMDBTVID`.
- Service (`syncChunk`), after `FetchAdaptations`:
  1. Collect the distinct work QIDs of the chunk and call `repo.PosterCandidates(ctx, qids) (map[string]bool, error)`. It returns true for a QID that is not stored yet, or stored with `poster_source IS NULL AND manually_edited_at IS NULL`. On error, skip posters for this chunk.
  2. Commons: for the candidates, collect the P3383 and P18 file names and resolve them with `FetchCommonsImages` in batches of ≤ `MaxEntitiesPerRequest`. Pick P3383 if usable, else P18. A file is usable when `Free`, `ThumbURL != ""`, and, if `AttributionRequired`, `Author != ""`.
  3. TMDB: for the remaining candidates, when a TMDB client is configured, look up at most `posterTMDBLookupCap = 100` per chunk. Series and animated series try the TV id first, other kinds the movie id first.
  4. Put `Poster *domain.PosterInput{Source, URL, Author, License, LicenseURL, SourceURL}` on each `WorkInput`.

  Every failure is logged at Warn level and swallowed: it does not change the outcome, the notes or the return error.
- The Commons capability is obtained like `AuthorSource`, by type assertion on `source` to an interface with `FetchCommonsImages`, which `*bookdata.WikidataClient` satisfies. TMDB is injected with a new option `service.WithTMDB(PosterLookup)`. `NewService` keeps its signature, and the variadic options are added.
- Repository: `SyncBook` returns `(SyncCounts{Added, Posters int}, error)`.
  - A new work is inserted with its poster columns.
  - For an existing work: `UPDATE screen_works SET poster_* = ?, poster_updated_at = now WHERE id = ? AND poster_source IS NULL AND manually_edited_at IS NULL`. Its `RowsAffected` is added to `Posters`.
  - A newly inserted work with a poster also counts.
- The author film sync passes `Poster: nil` and is otherwise unchanged. If it shares `upsertWikidataWork`, a nil poster must not touch the poster columns.
- `SyncItem.Posters`, `SyncReport.Posters` and `CatalogSyncResult.Posters` (`json:"posters"`) are summed like `Added`. `SyncCreatedBooks` (after a harvest) ignores the counter.

### D7. TMDB client
- New package `backend/internal/pkg/tmdb`. `Client.Poster(ctx, kind string /* "movie"|"tv" */, id string) (string, error)` calls `GET https://api.themoviedb.org/3/{kind}/{id}?language=ru-RU&api_key=<key>`. It returns `https://image.tmdb.org/t/p/w342` + `poster_path` when `poster_path` matches `^/[A-Za-z0-9_-]+\.(jpg|jpeg|png)$`, and `""` for a missing poster or a 404.
- Errors:
  - 401 returns `ErrUnauthorized`, and the service stops TMDB for the rest of the chunk and logs once;
  - 429 and 5xx return `ErrUnavailable`.
- Every error message MUST be built without the request URL, so the key never reaches logs. Do not wrap `*url.Error` as is.
- Pacing is 10 requests/s through a limiter: reuse the limiter type the Wikidata client uses if it fits, else a simple ticker. Each request has a 10 s timeout, and the body is limited to 64 KiB. `WithBaseURL` is for tests.
- Config: `TMDB_API_KEY`, empty by default. Without it no client is built. It is wired in `cmd/api/main.go` (admin sync) and `cmd/worker/main.go` (catalog job, harvest sync).

### D8. Author films
The poster is a property of the shared work. The author film endpoints return it for free from the same columns. The author page uses the same card component. The author film sync does **not** fetch posters in this change: biopic posters only appear when the work is also a book adaptation or an admin uploads one. A follow-up can reuse the resolver for `wikidata_author_films`.

### D9. TMDB notice
The notice goes in the public footer (`widgets/footer`) and is always shown. It is a site credit, not per-card noise. Text:
«Постеры фильмов — Wikimedia Commons и <a href="https://www.themoviedb.org">TMDB</a>. This product uses the TMDB API but is not endorsed or certified by TMDB.»

## API
- Public `GET /books/{id}/adaptations`, `GET /authors/{id}/screen-works`, and the admin list endpoints of both: every item gains `poster: null | {url, source: "manual"|"commons"|"tmdb", author, license, license_url, source_url}`.
- `GET /books/{id}/adaptations/verdicts`: `min_votes` is now 1.
- `GET /admin/screen-works/{id}` gains `poster` and `poster_removed: bool`. `PATCH` ignores posters.
- New `PUT /admin/screen-works/{id}/poster`:
  - admin role and bearer, `multipart/form-data` with the field `file`;
  - 200 returns the work detail;
  - 400 «file is required», 413 «poster is too large» (over 5 MiB, `coverfetch.MaxBytes`), 422 «poster must be a JPEG, PNG or WebP image», 404 «screen work not found», 401 and 403.
  - Add `"PUT /api/v1/admin/screen-works/:id/poster": 6 << 20` to `BodyLimitOverrides`. Check the key format against the router's route pattern.
  - Read at most `MaxBytes+1` from the part.
- New `DELETE /admin/screen-works/{id}/poster`: admin role and bearer, 200 with the detail, 404 and 403. It is idempotent.
- `POST /admin/books/sync-adaptations`: `posters` at the top level and per item. The catalog job result gains `posters`.
- New public file route `GET|HEAD /posters/:shard/:file`, outside `/api/v1` like `/covers`, not in swagger.
- Swagger annotations for every changed or new `/api/v1` route.

## Database
Forward-only migration `backend/migrations/000043_screen_work_posters.up.sql`, plus a `.down.sql` that drops the index and columns. Agents never run the down migration.
```sql
ALTER TABLE screen_works
    ADD COLUMN poster_source      VARCHAR(16)
        CHECK (poster_source IN ('manual', 'commons', 'tmdb', 'removed')),
    ADD COLUMN poster_url         TEXT,
    ADD COLUMN poster_path        VARCHAR(80),
    ADD COLUMN poster_author      TEXT,
    ADD COLUMN poster_license     VARCHAR(100),
    ADD COLUMN poster_license_url TEXT,
    ADD COLUMN poster_source_url  TEXT,
    ADD COLUMN poster_updated_at  TIMESTAMPTZ,
    ADD CONSTRAINT chk_screen_works_poster CHECK (
        (poster_source IS NULL OR poster_source = 'removed')
            AND poster_url IS NULL AND poster_path IS NULL
     OR poster_source = 'manual' AND poster_path IS NOT NULL AND poster_url IS NULL
     OR poster_source IN ('commons', 'tmdb') AND poster_url IS NOT NULL AND poster_path IS NULL);
CREATE INDEX idx_screen_works_poster_path ON screen_works (poster_path)
    WHERE poster_path IS NOT NULL;
```
Existing rows get NULL, which means no poster and eligible for the backfill. No data migration is needed.

## Frontend
**packages/api-client**
- `ScreenWorkPoster` type, and `poster: ScreenWorkPoster | null` on `BookAdaptation`, inherited by `AuthorFilm` and the admin types;
- `AdminScreenWorkDetail.poster` and `poster_removed`;
- `admin.screenWorks.uploadPoster(id, file: File)` with a FormData body (the fetcher already skips the JSON content type for FormData) and `admin.screenWorks.deletePoster(id)`;
- `posters` on the sync report and the catalog job result types.

**apps/web**
- `entities/screen-work`:
  - `ui/screen-work-poster.tsx` (client): a 2:3 frame with `next/image fill`, `unoptimized` for absolute http(s) URLs, `alt={title}`, and `onError` switching to the placeholder. The placeholder shows the kind icon and the year, using a `lib/kind-icon.ts` map with lucide icons: `Film` for film and tv_film, `Tv` for series, `Sparkles` for the animated kinds and `Drama` for teleplay. Icons are `aria-hidden`.
  - `ui/poster-attribution.tsx`: «Постер: …», modelled on `entities/author/ui/photo-attribution.tsx`. Do not import from `entities/author`.
  - `BookAdaptations` and `AuthorFilms` become a card grid, for example `grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4`. Each card has the poster, then the caption for commons, then the title, the meta line, and `renderExtra`. `BookAdaptations` gets `headingHidden` and `id="adaptations"`.
  - `verdictLabel` returns `string | null` (D1).
- `features/vote-adaptation/ui/adaptation-verdict.tsx`: the dropdown (D2), no `border-t`. `AdaptationVotePrompt` is unchanged; it is rendered at the top of the tab through `intro`.
- `views/book-detail`: `book-tabs.tsx`, `book-tab-link.tsx`, the header link, the `RatingSummary` links, the tab assembly, the `tab` prop and the canonical in `book-seo`/metadata (D3).
- `widgets/similar-books`: `emptyFallback` and `headingHidden`. `widgets/reviews-section`: `headingHidden`.
- `widgets/footer`: the TMDB notice.
- `next.config.js`: the `/posters` rewrite and the headers exclusion.

**apps/admin**
- `views/screen-work-edit` (or `features/edit-screen-work`): a new `features/screen-work-poster` slice with the «Постер» section. It has a hidden `<input type="file" accept="image/jpeg,image/png,image/webp">`, client checks on `file.type` and `file.size <= 5 * 1024 * 1024`, and `window.confirm` (or the existing confirm dialog pattern) for removal. It invalidates the work detail query after success.
- Sync banners (`features/*sync-adaptations*`): append the poster count (catalog-import spec).
- `next.config.js`: the `/posters` rewrite and the headers exclusion.

## Existing Patterns
- Commons resolution, licence allowlist and attribution: `bookdata/commons.go`, `entities/author/ui/photo-attribution.tsx`, migration 000016.
- Content-addressed file store, file route and cache headers: `pkg/coverstore`, `modules/covers/transport/http/handler.go`, and `coverfetch.SniffExt`/`MaxBytes`.
- Multipart handling and the body-limit override: `admin/transport/http/handler.go` (import) and `transport/http/limits.go`.
- Fill-empty upsert guarded by `manually_edited_at`: `screenworks/repository/repository.go` `upsertWikidataWork`.
- Optional capability by type assertion: `screenworks/service/service.go` `NewService` (`AuthorSource`).
- Env passthrough for optional API keys: `ANTHROPIC_API_KEY` in both compose files.

## Risks
- **SPARQL cost**: four more OPTIONALs on a 50-book query may raise WDQS time. Use `SAMPLE`, keep `GROUP BY ?book ?work`, and check the parser test fixtures. If live timeouts appear, lower the chunk only for this query in a follow-up.
- **TMDB reachability and terms**: api.themoviedb.org and image.tmdb.org may be slow or blocked from the future VPS region. The sync degrades silently, and broken images fall back to the placeholder. The terms also mention using the TMDB logo. The text notice with a link is the minimum, and adding the official logo asset is a follow-up before a real launch.
- **P18 is not always a poster**: it may be a film still or a photo. The admin removal (`removed`) covers wrong cases.
- **Privacy**: hotlinked images expose readers' IPs to Wikimedia and TMDB, as author photos already do.
- **Backups**: uploaded posters live only in the `posters-data` volume, and the backup script dumps only Postgres. Document it in `docs/DEPLOYMENT.md` as a parked prod item, like the other prod-only items.
- **API key in logs**: covered by D7. Tests must assert that an error string does not contain the key.
- **Works edited by hand get no automatic poster** (D5). This is deliberate and documented in the admin «Постер» section text.
- **`min_votes` contract value change**: only our web app consumes it. The curated verdict floor stays 5 (D1).
- **Local compose**: the new `posters-data` volume is added to the existing stack. Agents must not bring up a second compose stack (volume-collision history).
