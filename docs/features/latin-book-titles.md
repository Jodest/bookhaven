# Latin-Script Book Titles

Status: implemented

Product owner request (translated): allow Russian books whose official title is written in Latin script (e.g. «Empire V», «Generation П», «S.N.U.F.F.»). Today they are rejected with `ErrTitleNotRussian` by `validateBookInput` (`backend/internal/modules/admin/service/service.go`), per `docs/features/russian-only-content.md`.

## Goal

Narrow, explicit exemption from the Russian-title check (Cyrillic-ratio heuristic `IsRussianText`) for (1) the Wikidata harvest pipeline and (2) admin manual create/edit when the admin opts in per request. Bulk CSV/JSON import keeps the check. The description check is unchanged everywhere.

## User Stories

- As a content admin, I can create a book titled «Empire V» by ticking «Название на латинице допустимо» in the book form; without the checkbox the form still rejects Latin titles as today.
- As a content admin, I can later edit any other field of «Empire V» (or publish it) without re-ticking the box, because its title did not change.
- As a content admin running a Wikidata harvest, works whose Russian Wikidata label is in Latin script (e.g. «S.N.U.F.F.») are created as pending-review books instead of appearing in the job's error list.

## Design decisions (resolved)

1. **Per-request flag, not persisted. No migration.** A new `BookInput.AllowNonRussianTitle bool` field (zero value = check applies). The book row stores nothing new.
2. **Unchanged title is grandfathered on update.** The re-ticking problem is solved on the server, not with frontend heuristics: `UpdateBook` skips the title-language check when the trimmed input title equals the stored `book.Title`. This also covers paths that never send the flag: the single-row «Опубликовать» action (`books-list-view.tsx` `publishMutation` → `PATCH /admin/books/:id`) and `BulkPublishBooks` → `publishHarvestedBook` → `UpdateBook`. Without this rule, every Latin-titled harvested book would fail to publish. The check exists to stop *new* non-Russian titles from entering the catalog. A title that is already stored (whether it got in via the bypass or predates the russian-only rule) is not new text. **No frontend pre-tick and no frontend Cyrillic-ratio helper.** The checkbox always starts unchecked.
3. **The bypass is a service-input field, set only by trusted callers:**
   - harvest `processRecord`: always `true`
   - admin HTTP `CreateBook`/`UpdateBook`: copied from the request body
   - `import.go` `importRow`: never set (stays `false`). The import row parser must not map any column or key to it.
   - `bulk_publish.go` `publishHarvestedBook`: not set. Not needed because of the unchanged-title rule.
4. **The bypass covers the title only.** `ErrDescriptionNotRussian`, required-title, language, year, page-count, slug, ISBN, and author/genre checks all still apply when the flag is `true`.
5. **API field name: `allow_non_russian_title`** (boolean, optional, default `false`). The field is accepted on input only and never returned in `bookResponse`.

## Acceptance Criteria

**Backend: service (`backend/internal/modules/admin/service/service.go`)**
- [ ] `BookInput` gains `AllowNonRussianTitle bool`. Add a doc comment saying it lifts only the `ErrTitleNotRussian` check, is set by the harvest pipeline and the admin HTTP handlers, and is never set by bulk import.
- [ ] `validateBookInput` takes a way to skip the title-language check, e.g. `validateBookInput(input BookInput, skipTitleLangCheck bool) error`. The developer may pick a different shape as long as the skip decision is made by the caller. Only the `IsRussianText(title)` → `ErrTitleNotRussian` check becomes conditional. The empty-title (`ErrInvalidTitle`) and description checks stay unconditional, and the check order is unchanged.
- [ ] `CreateBook` passes `input.AllowNonRussianTitle`.
- [ ] `UpdateBook` passes `input.AllowNonRussianTitle || strings.TrimSpace(input.Title) == book.Title`. The book is already loaded before validation, so no extra query is needed. Add a comment explaining the grandfathering rule (Design decision 2).
- [ ] No new sentinel errors. `ErrTitleNotRussian`'s message (`"title must be in Russian"`) stays exactly as it is, because the frontend matches on it.

**Backend: admin HTTP (`backend/internal/modules/admin/transport/http/handler.go`)**
- [ ] `createUpdateBookRequest` gains `AllowNonRussianTitle bool \`json:"allow_non_russian_title"\``.
- [ ] `CreateBook` and `UpdateBook` handlers pass `AllowNonRussianTitle: req.AllowNonRussianTitle` into `BookInput`. Error mapping is unchanged: `ErrTitleNotRussian` is still 422.
- [ ] Swagger annotations pick the new field up from the struct automatically. Regenerate swagger docs if the repo commits them.

**Backend: harvest (`backend/internal/modules/harvest/service/handler.go`)**
- [ ] `processRecord` sets `AllowNonRussianTitle: true` in the `BookInput` it builds. Add a comment: the title comes only from the Wikidata `ru` label (`FILTER(LANG(?itemLabel) = "ru")` in `backend/internal/pkg/bookdata/wikidata.go`), so it is the official Russian title by definition even when written in Latin script.
- [ ] The description handling is unchanged: `enrichRecord` still silently drops a non-Russian Open Library description, and `validateBookInput`'s description check still applies.
- [ ] Update the `enrichRecord` comment that currently says the title is "hard-rejected via validateBookInput (ErrTitleNotRussian)". It should now say harvest titles are exempt (`ru`-label by construction) and point to this contract.

**Backend: bulk import / bulk publish**
- [ ] `import.go`: no code change. `importRow`'s `BookInput` must not set `AllowNonRussianTitle`. Lock this in with a test (see Tests).
- [ ] `bulk_publish.go`: no code change. Latin-titled pending-review books publish through the unchanged-title rule.

**API client (`packages/api-client/src/index.ts`)**
- [ ] `AdminBookInput` gains `allow_non_russian_title?: boolean`.

**Admin frontend (`apps/admin/src/features/manage-book/ui/book-form.tsx`)**
- [ ] Add a native `<input type="checkbox" id="allow_non_russian_title">` with a `<Label>` «Название на латинице допустимо» directly under the Title field. `@bookhaven/ui` has no Checkbox component, so use the native input; do not add a UI-kit component for this. Register it via `register('allow_non_russian_title')`. Default value is `false` in both create and edit modes (no pre-tick).
- [ ] Add helper text under the checkbox (muted, `text-xs`): «Только для книг, официальное название которых пишется латиницей (например, «Empire V»). При редактировании без изменения названия отмечать не нужно.»
- [ ] Error mapping in `handleFormSubmit`: when `error.status === 422 && error.message === 'title must be in Russian'`, show «Название должно быть на русском языке. Если официальное название книги пишется латиницей, отметьте «Название на латинице допустимо».» instead of the raw English message. Keep the matched string in a local constant. All other 422 messages keep today's pass-through behavior.
- [ ] The books-list single-row publish action (`apps/admin/src/views/books-list/ui/books-list-view.tsx`) needs no change. It never sends the flag and relies on the unchanged-title rule.

## Business Rules

- Title-language check matrix:

  | Path | Title check |
  |---|---|
  | Wikidata harvest (`processRecord`) | skipped always |
  | Admin create (`POST /admin/books`) | applied unless `allow_non_russian_title: true` |
  | Admin update (`PATCH /admin/books/:id`) | applied only if the trimmed title differs from the stored title and `allow_non_russian_title` is not `true` |
  | Single-row publish / bulk publish | via update → title unchanged → skipped |
  | CSV/JSON bulk import | always applied (no bypass) |

- Description check (`ErrDescriptionNotRussian`) is applied on every path regardless of the flag.
- The flag is not persisted and has no meaning outside the request that carries it.
- Authorization is unchanged. The admin book endpoints are already admin-role-only and Bearer-only for state-changing requests. The flag gives no extra privilege beyond what an admin already has.

## API

`POST /api/v1/admin/books` and `PATCH /api/v1/admin/books/:id` accept a new optional body field:

```json
{ "allow_non_russian_title": true }
```

Default is `false`. The response shape is unchanged. The 422 `{"error":"title must be in Russian"}` is still returned when the check applies and fails. `POST /api/v1/admin/books/import` is unchanged, and import rows have no bypass.

## Database

None.

## Frontend

See the Admin frontend acceptance criteria. This affects only the `features/manage-book` slice and the `packages/api-client` type. The web app is untouched.

## Existing Patterns

- The `BookInput` field-set-by-trusted-caller pattern already exists for `ExternalSource`/`ExternalID`, which are set by harvest and not exposed via HTTP. `AllowNonRussianTitle` follows the same shape, but is also exposed via HTTP.
- The `book-form.tsx` `handleFormSubmit` status/message mapping already exists (409 → Russian slug message). Extend it with the one new message match.

## Tests

- `backend/internal/modules/admin/service/service_test.go`:
  - `CreateBook` with a Latin title and `AllowNonRussianTitle: true` succeeds.
  - `CreateBook` with a Latin title and no flag returns `ErrTitleNotRussian`. The existing test stays.
  - `CreateBook` with `AllowNonRussianTitle: true`, a Latin title and a non-Russian description returns `ErrDescriptionNotRussian`.
  - `CreateBook` with `AllowNonRussianTitle: true` and an empty title returns `ErrInvalidTitle`.
  - `UpdateBook` on a stored Latin-titled book with the same title and no flag succeeds (grandfathering).
  - `UpdateBook` changing the title to a different Latin title without the flag returns `ErrTitleNotRussian`, and with the flag succeeds.
  - Update the existing `UpdateBook` "Updated Book" test (~line 458) so the stored title differs from the input. Otherwise it passes now through the grandfather rule.
- `backend/internal/modules/admin/service/bulk_publish_test.go` (or the existing bulk-publish test file): a pending-review book with title «Empire V» publishes successfully.
- `backend/internal/modules/admin/service/import_test.go`: the existing non-Russian-title row test still lands in `ImportReport.Errors`, which guards that import never bypasses. If import parsing accepts JSON objects, also confirm that a JSON row containing `"allow_non_russian_title": true` is still rejected.
- `backend/internal/modules/admin/transport/http/handler_test.go`: extend the fake service to capture the last `BookInput` for create/update. Assert that `allow_non_russian_title: true` in the request body arrives as `AllowNonRussianTitle == true`, and that an omitted field arrives as `false`. The existing 422 mapping tests stay.
- `backend/internal/modules/harvest/service/handler_test.go`:
  - Replace `TestRunHarvest_NonRussianTitlePastSPARQLFilterIsAPerRecordError` (~line 574) with a test asserting that a Latin-title record (e.g. `"S.N.U.F.F."`) is created (`Created == 2`, `len(Errors) == 0`).
  - Add a `processRecord` test asserting the `BookInput` passed to `CreateBook` has `AllowNonRussianTitle == true`, either directly or through the created book's title.
  - `TestProcessRecord_NonRussianEnrichmentDescriptionIsDropped` must keep passing unchanged.
- `apps/admin/src/features/manage-book/ui/book-form.test.tsx`:
  - The checkbox renders unchecked in both create and edit modes.
  - Ticking it submits `allow_non_russian_title: true`.
  - A 422 `"title must be in Russian"` shows the Russian hint message mentioning the checkbox.
  - Another 422 message is still shown as is.
- Go checks run via Docker (`docker compose run`). Frontend: `tsc`/vitest natively. CI is the gate for `next build`.

## Documentation

- This file: set `Status: implemented` when done.
- `docs/features/russian-only-content.md`, Business Rules: append a new bullet after the "Failure handling" bullet:
  > - **Latin-script official titles exemption** (added by `docs/features/latin-book-titles.md`): the Title check (`ErrTitleNotRussian`) is not applied to (a) Wikidata harvest records, since the title is taken only from the `ru` label and is the official Russian title even when written in Latin script (e.g. «Empire V», «S.N.U.F.F.»); (b) admin create/update requests with `allow_non_russian_title: true` (checkbox «Название на латинице допустимо»); (c) admin updates, including single/bulk publish, that leave the stored title unchanged. CSV/JSON bulk import has no bypass. The Description check (`ErrDescriptionNotRussian`) is unaffected on every path.

  In the same file, amend the harvest sub-bullet under "Failure handling" ("Harvest: `Title` failures become a per-record entry...") and the "defense-in-depth" sentence in the SPARQL-fix bullet to say harvest titles are now exempt (see the exemption above). Do not otherwise rewrite the historical contract.
- `docs/ROADMAP.md`, "Completed": append an `[x]` line when merged, e.g. `- [x] Latin-script book titles — harvest and an admin opt-in checkbox exempt official Latin-script titles (e.g. «Empire V») from the Russian-title check; bulk import keeps it (see [docs/features/latin-book-titles.md](features/latin-book-titles.md))`.

## Risks

- **Harvest loses its defense-in-depth title check.** A `ru`-tagged Wikidata label that is really an untranslated English title now gets in as a pending-review book. Mitigation: harvested books are created hidden (`IsHidden: true`) and require admin publish. Bulk publish makes this review cheap to skip, which is accepted.
- **Grandfathering also covers pre-existing non-Russian titles.** Any legacy book stored before the russian-only rule can be edited or published without the checkbox. This is intended: the rule guards new text.
- **Comparison is exact after trim.** Fixing a typo inside a Latin title counts as a title change, so the admin must tick the box. This is acceptable and the helper text covers it.
- **Frontend matches the backend English error string.** If `ErrTitleNotRussian`'s message ever changes, the hint silently falls back to the raw message. Mitigated by the frontend test.
- **The admin can tick the box for an English-titled foreign book** (e.g. «Brave New World»). This is a deliberate per-book admin decision. The checkbox label and helper text set the expectation, and there is no further server-side restriction.
