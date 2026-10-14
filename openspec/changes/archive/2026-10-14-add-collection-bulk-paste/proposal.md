# Proposal

Epic #413, feature 2 of 5.

## Why

Filling a «Топ-100» or «Топ-200» curated collection one catalog search at a time is impractical. Admins need to paste a ready list of books, see how each line matched the catalog, fix the ambiguous ones and add everything in one step.

## What Changes

- New read-only preview endpoint: an admin pastes up to 500 lines («Название — Автор», numbering, quotes, tab-separated or title-only). Each line is matched against the catalog: «Найдено», «Неоднозначно» (with candidates), «Не найдено», «Отклонена», «Уже в подборке», «Повтор», «Не распознано». Nothing is written.
- New bulk-add endpoint: appends the chosen catalog books in the given order, at the end or at a chosen position. It runs in one transaction under the existing collection lock and respects the 1000-book cap (all or nothing on capacity). It reports what was added and what was skipped and why.
- A pasted line never creates a book or a placeholder. Unmatched lines are reported as «Не найдено — сначала добавьте книгу в каталог».
- Admin UI: a «Вставить список» panel on the collection edit page. It has a textarea, a preview table with checkboxes and candidate pickers, a position choice, «Добавить выбранные (N)», a result report, and copying of the lines that were not found.
- Forward-only migration: a word index on normalized book titles, so matching stays fast.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `collections/curated-collection-admin`: adds bulk paste preview, line parsing and catalog matching rules, bulk add, and the paste panel of the admin edit page.

## Impact

- backend: `curated` module (paste parsing and matching, preview and bulk-add service methods, repository candidate lookup and bulk insert, two admin routes, swagger); shared text normalizer extracted to `internal/pkg`; migration `000026`.
- frontend: `packages/api-client` (types and two methods); `apps/admin` new feature `paste-collection-books`, and the `collection-books` widget gets a «Вставить список» entry point.
- devops: none (one forward migration, applied by the usual migrate step).
- docs: OpenSpec delta only; swagger regenerated.
