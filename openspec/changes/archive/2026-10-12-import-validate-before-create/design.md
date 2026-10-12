## Context

`ImportBooks` (`backend/internal/modules/admin/service/import.go`) calls `importRow` once per row. Today `importRow` works in this order:

1. It calls `resolveOrCreateAuthorsByNames` and returns `errImportNoAuthors` if no IDs come back.
2. It calls `resolveOrCreateGenresByNames`.
3. It calls `CreateBook` (`service.go`). `CreateBook` checks the status, then `validateBookInput`, slug derivation and `isURLSafeSlug`, `BookSlugExists` (`ErrDuplicateSlug`, which is skip), the ISBN checksum, the author/genre ID checks, and finally `repo.CreateBook`. `repo.CreateBook` already runs in its own GORM transaction.

So authors and genres are written before any book check. Each repository call runs on `r.db` in its own transaction, and the admin repository has no transaction that spans several calls.

## Decisions

1. **Pre-check first, using the same code as `CreateBook`.** Extract the checks that run before the UUID parsing in `CreateBook` into one unexported helper. Name it, for example, `prepareNewBook(ctx, input BookInput) (preparedBook, error)`. It runs the status check, `validateBookInput`, the slug derivation and safety check, `BookSlugExists` (returns `ErrDuplicateSlug`), and the ISBN normalization and checksum, in exactly today's order. It returns the derived slug, the status and the normalized ISBN. `CreateBook` calls it, so there is still a single source of truth and the single-book create behaves as before. `importRow` calls it before resolving any name. The order inside the helper must stay as it is: the slug check runs before the ISBN check, so a row with an existing slug and an invalid ISBN stays `skipped_existing`, as today.
   - *Implementation note:* in `CreateBook` the ISBN check runs **after** the author/genre ID checks, not before them. Putting it inside `prepareNewBook` would move it ahead of those checks and change the single-book error precedence (e.g. invalid author IDs plus an invalid ISBN). So the helper is split in two: `prepareNewBook` covers status → `validateBookInput` → slug → `BookSlugExists` and returns the status and slug, and `normalizeNewBookISBN` covers the ISBN normalization and checksum. `CreateBook` calls `prepareNewBook`, then the ID checks, then `normalizeNewBookISBN`, which is exactly today's order. `importRow` calls `prepareNewBook` and then `normalizeNewBookISBN` before any write. For a row, the ID checks cannot fail, because the IDs come from the resolvers, so the row outcome is the same.
2. **The no-authors check needs no DB.** A non-blank trimmed name always resolves, either by match or by creation, unless there is a DB or creation error. So "has a resolvable author" is checked up front as "at least one non-blank name after `strings.TrimSpace`" and keeps returning `errImportNoAuthors`. It runs before `prepareNewBook`, so the error precedence stays the same: no authors, then book validation, then skip.
3. **One transaction per row, which is feasible and chosen.** The invasive part is small:
   - Add `WithinTransaction(ctx context.Context, fn func(repo Repository) error) error` to `repository.Repository`. The `GormRepository` implementation is `r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return fn(&GormRepository{db: tx}) })`. Every repository method already starts from `r.db.WithContext(ctx)`, so a repository bound to `tx` runs every call in the transaction. The nested `Transaction` in `repo.CreateBook` becomes a SAVEPOINT. Nested transactions are GORM's default, and the codebase does not disable them.
   - Add `func (s *Service) withRepo(repo repository.Repository) *Service { c := *s; c.repo = repo; return &c }`. This is a shallow copy; `Service` holds no mutex or per-call state. The row then runs `resolveOrCreate…` and `CreateBook` on the copy, unchanged.
   - `importRow` flow:
     1. Run the no-authors check, then `prepareNewBook`. An error is a row error and a duplicate is skipped. Nothing has been written yet.
     2. Call `s.repo.WithinTransaction`. Inside it, resolve the authors and the genres, then call `CreateBook`. Any error returned from `fn` rolls the whole row back, including `ErrDuplicateSlug` from a concurrent insert.
     3. Classify after the transaction: `ErrDuplicateSlug` is skipped, another error is a row error, and success is created.
   - `importRow` returns the created entities **only on the created outcome**. On error or skip it returns an empty `importRowCreated`, so a rolled-back entity is never reported. The `ImportBooks` loop and its dedup by ID stay as they are. Update the doc comments on `ImportReport.CreatedAuthors` and `importRow`, which currently say failed or skipped rows report entities.
   - The resolvers' "return created-so-far together with the error" behavior stays because it is harmless. It is just not used for reporting on the error path any more.
   - The fallback, a non-transactional pre-check only, is not chosen. It would leave orphans on post-check failures, such as the second author's creation failing or a DB error. The transaction costs one interface method plus four one-line fake implementations.
4. **`CreateBook` re-runs the pre-checks inside the transaction.** This is harmless: the checks are cheap, and the second `BookSlugExists` narrows the race window. Do not add a "skip checks" flag.
5. **No change to the public resolver signatures, harvest, or the single-book create.** `WithinTransaction` is used only by the file import.

## API

`POST /api/v1/admin/books/import`: the route, authorization (`admin` role and bearer), status codes and the report shape are unchanged. Only the content of `created_authors` and `created_genres` changes. Swagger needs no regeneration unless a doc comment on a swagger-annotated type changed. If the `ImportReport` field comments feed swagger descriptions, regenerate with `docker compose exec backend swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`.

## Database

No migration. The change only wraps existing writes in one transaction per row. Each row's transaction is short-lived: a few lookups and at most a few inserts. Never run destructive commands such as `migrate down`, drop or reset.

## Existing Patterns

- Multi-step transactions already live in the admin repository (`CreateBook`, `UpdateBook`, `MergeGenres` in `genre_moderation.go`, `ChangeUserRole` in `users.go`). `WithinTransaction` generalizes them for a service-level unit of work.
- SQLite-backed repository tests (`admin/repository/*_sqlite_test.go`, for example `newGenreStatusRepo`) are the place to prove that commit and rollback work against a real driver.
- `fakeImportRepository` in `import_test.go` is the dynamic fake for import tests. It already has a `createAuthorErr` hook.

## Risks

- **Fakes must implement the new interface method.** These are `fakeRepository` (`service_test.go`), `fakeMergeAuthorsRepository` (`merge_author_test.go`), `fakeImportRepository` (`import_test.go`) and `fakeAdminRepository` (`harvest/service/handler_test.go`). For all except the import fake, `return fn(f)` is enough. `fakeImportRepository` must simulate a rollback: copy `books`, `authors` and `genres` before calling `fn` and restore them if it returns an error. Otherwise the rollback tests prove nothing.
- **Concurrent imports naming the same new author.** The second transaction blocks on the unique slug until the first commits, then gets a duplicate key. `createAuthor` maps that to `ErrDuplicateSlug`, which `importRow` would classify as **skip**, because the resolver wraps it with `%w`. Today this is a row error, because `importRow` checks `ErrDuplicateSlug` only on `CreateBook`. Keep that distinction: classify a skip only when `ErrDuplicateSlug` comes from `CreateBook`. For example, wrap resolver errors in a distinct row-error type, or call `CreateBook` last and remember which step failed. Add a test for this.
- **A pre-check that diverges from `CreateBook`** would change row outcomes. Decision 1, one shared helper, prevents this. Existing `CreateBook` tests in `service_test.go` must pass unchanged.
- **Lock duration.** At most 500 short transactions run sequentially in one request, with no external calls inside them (the file import makes no network calls). This is acceptable.
