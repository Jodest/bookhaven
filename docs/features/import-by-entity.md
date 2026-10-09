# Import by Entity (F1)

Status: done

Tracking: issue #314, epic #310 (admin catalog/import/moderation unification).
Scope: `apps/admin` only. No backend or api-client changes.

## Goal

The admin "Импорт" section is organized by entity (Книги / Авторы / Жанры, plus Задачи).
Every way to add an entity lives on that entity's import tab. Creating from the Catalog
is removed, so creation exists in only one place. Only existing functionality is
moved. Nothing new is built, and a capability an entity lacks is left out (no
placeholders).

## User Stories

- As an admin, I open Импорт → Книги and find every way to add books: manual form, Wikidata harvest by author, CSV/JSON file.
- As an admin, I open Импорт → Авторы and either create an author manually or import one from Wikidata.
- As an admin, I open Импорт → Жанры and either create a genre manually or import one from Wikidata.
- As an admin with an old bookmark (`/import/wikidata`, `/import/file`, `/import/author`, `/catalog/books/new`), I land on the right new tab.
- As an admin in Каталог, I still edit books/authors/genres exactly as before, but there is no "add" there.

## Acceptance Criteria

- [x] Импорт tabs are, in order: Книги `/import/books`, Авторы `/import/authors`, Жанры `/import/genres`, Задачи `/import/jobs`. All are `roles: ['admin']`. `/import` redirects to `/import/books`.
- [x] `/import/books` is one page with stacked sections, in this order: «Вручную» (BookForm create), «Харвест из Wikidata» (current harvest trigger), «Из файла» (current CSV/JSON import).
- [x] `/import/authors` has the sections «Вручную» (AuthorForm create) and «Wikidata» (current author import). It has no file section.
- [x] `/import/genres` has the sections «Вручную» (GenreForm create) and «Wikidata» (current genre search/import). It has no file section.
- [x] `/import/jobs` is unchanged.
- [x] Each section works exactly as before: same API calls, validation, feedback banners and query invalidations.
- [x] Manual creation stays on the Import page. The form resets and a success banner appears with a link to the created record (see UX).
- [x] Catalog → Книги no longer has the «Добавить книгу» button.
- [x] Catalog → Авторы / Жанры no longer show a create form. The AuthorForm/GenreForm appears only after «Редактировать» is clicked and disappears after save or «Отмена». Edit, status change, merge (authors) and the QID 409 hint (genres) all keep working.
- [x] `/import/wikidata` and `/import/file` redirect to `/import/books`, `/import/author` redirects to `/import/authors`, and `/catalog/books/new` redirects to `/import/books`. `/catalog/books/[id]/edit` is unaffected.
- [x] No internal link, test fixture or code comment points to the removed URLs (grep `import/wikidata|import/file|import/author|catalog/books/new` returns only the `redirects()` entries and this doc and other docs).
- [ ] `tsc --noEmit`, `next lint` (FSD boundaries) and `vitest` pass (done locally). CI build (typedRoutes) is green (pending CI).

## Business Rules

1. Creation is available only under Импорт. Catalog keeps list, search, filter, edit and merge.
2. Manually created authors and genres are still created `published` by the server, and their forms still hide the status field when creating (unchanged). A book's status select stays as-is.
3. Role gating is unchanged: every Import tab is admin-only. The frontend guard is UX only, and the backend role checks stay authoritative.
4. Moving code must not change any request payload or error mapping.

## UX

- **Layout pattern (same on all three entity tabs):** the page h1 is «Импорт: книги» / «Импорт: авторы» / «Импорт: жанры». Below it are stacked bordered sections, each with an h2 title and a one-line muted description, in the fixed order Вручную → Wikidata → Файл (absent ones are skipped). Stacked sections were chosen over an in-page switch because they hide no state, keep each section's feedback banner independent, need no query param or typed route, and keep tests simple.
- The moved components lose their own `<h1>`, because the section h2 replaces it. Their inner layout stays the same.
- `BookImportView`'s «К списку книг» button is removed. It was page-level chrome, and inside a section it is noise.
- **After manual creation (all three entities), stay on the page:**
  - Book: form resets to create defaults, and a green banner «Книга «{title}» создана» appears with the link «Открыть» → `/catalog/books/{id}/edit`. Invalidate `['admin','books']` (check the key used by books-list).
  - Author: form clears (existing behavior), and the banner «Автор создан» gets the link «Открыть в каталоге» → `/catalog/authors?search={name}`. There is no author detail page until F3.
  - Genre: form clears, and the banner «Жанр создан» gets the link «Открыть в каталоге» → `/catalog/genres?search={name}`.
  - Errors show inline, as today (BookForm serverError, author errorMessage, genre 409-QID hint).
- Copy fixes: the genre import empty hint changes from «…создайте жанр вручную в каталоге.» to «…создайте жанр вручную в разделе «Вручную».» The author import hint gets the same treatment: «…или создайте автора вручную в разделе «Вручную».»
- Catalog authors/genres pages: the update success banner («Автор обновлен» / «Жанр обновлен») renders at page level, because the form unmounts after save. The catalog gets no replacement "add" link.

## API

No changes. These existing endpoints are reused: `adminBooksApi.create/import`, `adminHarvestApi.trigger`, `adminAuthorsApi.create`, `adminGenresApi.create`, the Wikidata author/genre search/import hooks, and all `create` calls returning the created entity (`AdminBook` / `AdminAuthor` / `AdminGenre`).

## Database

None.

## Frontend

FSD constraint: views cannot import other views (`.eslintrc.json` boundaries). Reusable page bodies therefore move down to `widgets`, and new entity views compose them.

**Moves (logic unchanged; drop the h1; update the file-header comments' URLs):**

| From | To (widget slice, public `index.ts`) |
|---|---|
| `views/harvest-trigger` (incl. `author-name-search.tsx` + test) | `widgets/book-harvest` → `BookHarvestPanel` |
| `views/book-import` | `widgets/book-file-import` → `BookFileImportPanel` (without «К списку книг») |
| `views/author-import` | `widgets/wikidata-author-import` → `WikidataAuthorImportPanel` |
| `views/genre-import` | `widgets/wikidata-genre-import` → `WikidataGenreImportPanel` |

Delete `views/book-new` (replaced by the book create section).

**New:**
- `shared/ui` `PageSection` (`title`, `description?`, `children`): a `<section aria-labelledby>` with h2 and border. Export it from `shared/ui/index.ts`.
- `views/import-books`: `ImportBooksView` with a slice-local `ui/book-create-section.tsx` (BookForm + `adminBooksApi.create`, success banner state), followed by `BookHarvestPanel` and `BookFileImportPanel`.
- `views/import-authors`: `ImportAuthorsView` with `ui/author-create-section.tsx`, which is the create half lifted from `AuthorsView`: `formData` state, `createMutation`, `<AuthorForm editing={null} …>` with success/error. Below it, `WikidataAuthorImportPanel`.
- `views/import-genres`: `ImportGenresView` with `ui/genre-create-section.tsx` (create half of `GenresView`), then `WikidataGenreImportPanel`.
- Extract the genre 409-QID error text (currently inline in `GenresView`) into `features/manage-genre` as `genreMutationErrorText(err)`, so the create section and the catalog edit share it.

**BookForm (`features/manage-book`):** add an optional prop `stayOnSuccess?: boolean` (default `false`).
- When `true`, a successful `onSubmit` calls `reset()` to create defaults instead of `router.push('/catalog/books')`, and the «Отмена» button is not rendered.
- When `false`, the behavior is exactly as today, so `BookEditView` is unchanged.
- The view captures the created book from its own `onSubmit` (`const book = await adminBooksApi.create(data)`) to build the banner.

**Catalog edits:**
- `views/books-list`: remove the «Добавить книгу» `Link`/`Button` and the now-unused `Plus` import.
- `views/authors`: remove `createMutation`. `handleSubmit` only updates. Render `<AuthorForm>` only when `editing !== null`. Move the update success banner out of the form to page level (pass `successMessage={null}` to the form or render it outside). `errorMessage` comes from `updateMutation` only. Merge dialog is untouched.
- `views/genres`: remove `createMutation`. Render `<GenreForm>` only when `editing !== null`. Keep the page-level success/error banners and use `genreMutationErrorText`.
- `AuthorForm`/`GenreForm` components themselves are unchanged. They already switch on `editing`, and both the catalog (edit) and the import page (create) use them.

**App routes:**
- Add `app/import/books/page.tsx` → `ImportBooksView` and `app/import/authors/page.tsx` → `ImportAuthorsView`.
- Change `app/import/genres/page.tsx` → `ImportGenresView`.
- Delete `app/import/wikidata`, `app/import/file`, `app/import/author` and `app/catalog/books/new`.
- No dynamic routes are added.

**Redirects:** use `async redirects()` in `apps/admin/next.config.js`, with 4 entries, `permanent: false` (later epic phases may move things again):
`/import/wikidata`→`/import/books`, `/import/file`→`/import/books`, `/import/author`→`/import/authors`, `/catalog/books/new`→`/import/books`.
- Config redirects need no page files.
- They are compatible with `typedRoutes`, since redirect sources are only added to the route union and no code links to them.
- No redirect is needed for `/import/genres` (same URL).

**Navigation (`shared/config/navigation.ts`):** replace the import tabs with:
- `import.books` (`/import/books`, match `['/import/books']`)
- `import.authors` (`/import/authors`, match `['/import/authors']`)
- `import.genres` (`/import/genres`, match `['/import/genres']`)
- `import.jobs` (unchanged)

All are `roles: ['admin']`. `basePath: '/import'` stays.

**Links to update:**
- `views/dashboard/ui/dashboard-view.tsx:146` (`/import/wikidata` → `/import/books`)
- comment in `features/search-wikidata-author/ui/wikidata-author-search.tsx:27`
- moved views' header comments

## Existing Patterns

- Wikidata import pages (`views/author-import`, `views/genre-import`) show the dismissible `role="status"` banner with links. Reuse that markup for the create success banners.
- `SectionIndexRedirect` (`widgets/section-tabs`) handles `/import` automatically from the nav config.
- `genreModerationHref` in `features/import-wikidata-genre` is a link-builder precedent.

## Tests

Move these with their components and adjust for the h1 removal:
- `harvest-trigger-view.test.tsx`
- `author-name-search` tests, if any
- `book-import-view.test.tsx`
- `author-import-view.test.tsx`
- `genre-import-view.test.tsx`

New:
- `import-books-view.test.tsx`: three section headings render in order. A successful manual create calls `adminBooksApi.create`, shows the banner with the `/catalog/books/{id}/edit` link, resets the form and does not call `router.push`.
- `import-authors-view.test.tsx`: two sections with no file section. A create success shows the banner + catalog link and clears the form. An API error shows inline.
- `import-genres-view.test.tsx`: two sections. A create success shows the banner. A 409 QID error shows the «(возможно, у слитого жанра)» hint. Move the create cases from `genres-view.test.tsx` (≈ lines 116, 157, 165) here.
- `book-form.test.tsx`: `stayOnSuccess` resets the form, hides «Отмена» and skips push. The default still pushes `/catalog/books`.
- `PageSection` gets a small render test (optional).

Update:
- `authors-view.test.tsx`: the create test (≈ line 111) moves to import-authors. New assertions:
  - there is no «Создать» button or «Имя *» field initially
  - «Редактировать» shows the form with «Сохранить»
  - after save the form is gone and «Автор обновлен» is visible
  - merge still works
- `genres-view.test.tsx`: the same edit-only assertions. The 409 hint still applies on update.
- `books-list-view.test.tsx` (≈ line 176): assert that there is no «Добавить книгу» link.
- `shared/config/navigation.test.ts`:
  - the resolve table uses `/import/books|authors|genres|jobs`
  - `/catalog/books/new` → replace with `/catalog/books/123/edit`
  - the `/import/wikidatax` negative case → `/import/booksx`
  - `getSectionHref(import)` → `/import/books`
- `widgets/section-tabs/ui/section-tabs.test.tsx` (≈ lines 59-67, 124, 167-171): the tab list becomes Книги/Авторы/Жанры/Задачи, and `/import` → `/import/books`.
- `widgets/admin-shell/ui/shell.test.tsx` (≈ lines 77, 93-99, 131, 218): `/import/wikidata`/`/import/file` → new URLs.
- `dashboard-view.test.tsx:211`: `/import/books`.
- `author-moderation-view.test.tsx:326`: update the test title text only (`/import/authors`).

Run natively: `tsc --noEmit`, `next lint`, `vitest`. Do not run native `next build`, because CI is the typedRoutes gate.

## Documentation

- `docs/features/admin-navigation-restructure.md`: update the Import tab list/URLs, with a pointer to this doc.
- `docs/features/author-direct-import.md`, `docs/features/genre-wikidata-import.md`, `docs/features/catalog-manual-import.md`: replace the old URLs (`/import/author`, `/import/file`, `/catalog/books/new`, the catalog inline-create mentions) with the new tabs.
- `docs/ROADMAP.md`: mark F1 (#314) when done.
- Set this doc's status to `done` on merge.

## Risks

- **next.config.js change:** the dockerized admin dev server must be restarted to pick up `redirects()`.
- **Catalog edit regression:** AuthorForm/GenreForm are shared between create and edit, and the success banner currently lives inside AuthorForm. Hiding the form when not editing would hide «Автор обновлен» unless it is moved to page level. Covered by the tests above.
- **Duplicate DOM ids:** none found today. AuthorForm/GenreForm use `name`/`slug`/`wikidata_qid`, and the Wikidata searches use distinct `inputId`s. BookForm ids do not collide with `harvest-*`/`import-*`. Keep it that way when composing.
- **Long `/import/books` page:** BookForm is tall, so the harvest and file sections sit below the fold. This is accepted for consistency, and in-page anchors are out of scope.
- **typedRoutes:** deleted routes break any leftover `href` only in CI build (`tsc` misses typedRoutes). Grep the old URLs before pushing.
- **FSD boundaries:** a view importing another view fails lint, which is why the import page bodies move to widgets.
- **Out of scope:** catalog redesign/edit pages (F3 #316), moderation (F2/F4), backend, author/genre file import, author/genre harvesters.
