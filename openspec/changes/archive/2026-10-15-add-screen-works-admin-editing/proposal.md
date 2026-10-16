# Proposal

## Why
F3, the last part of the screen works series. Today a book or an author only gets films from Wikidata, so books and authors added by hand never get any. Wrong Wikidata data and a wrong biopic/documentary relation can only be hidden, not fixed. Admins need to add, fix and remove works and links by hand.

## What Changes
- Admins can add a film to a book (adaptation) or to an author (film about the author). They either pick an existing work or create a new one with title, original title, type, year, directors, IMDb id, Kinopoisk id and an optional Wikidata QID. A QID that is already stored links the existing work instead of creating a duplicate. Manual links are `source=manual` and are published at once, as in F1/F2.
- Admins can edit a work's data. Every manual create or edit marks the work as edited by hand, and from then on the Wikidata sync never changes it. This rule was laid down in F1.
- `PATCH /admin/authors/{id}/screen-works/{screen_work_id}` also accepts `relation` (`biopic` / `documentary`). `hidden` is no longer required on its own; at least one of the two fields is required.
- Admins can delete manual links. Wikidata links cannot be deleted (409), only hidden, so the sync cannot bring them back.
- New admin page «Фильмы и сериалы» (`/catalog/screen-works`) lists all works, with search and an «unlinked» filter. Its edit page shows the linked books and authors and edits the work. A work can be deleted only when it has no links (409 otherwise).
- The «Экранизации» and «Фильмы об авторе» sections on the book and author edit pages gain «Добавить…», «Удалить» (manual links only), «Изменить», the mark «Вручную», and a relation switch for author films.
- No migration: the existing schema (000029, 000030) already allows manual works and links.

## Capabilities

### New Capabilities
- `screen-works/admin-editing`: the work list and edit endpoints and pages, manual book and author links (create and delete), validation of work data, the manual-edit lock against the sync, and work deletion.

### Modified Capabilities
- `screen-works/book-adaptations`: the «Экранизации» section on the book edit page gains add, delete and edit actions and the «Вручную» mark.
- `screen-works/author-films`: the admin author film PATCH also changes `relation`. The «Фильмы об авторе» section gains add, delete, edit and relation actions.

## Impact
- backend: `screenworks` module (domain validation, repository, service, transport), router (new `/admin/screen-works` group, POST/DELETE link routes), Swagger.
- frontend: `packages/api-client`; `apps/admin`: a new entity `screen-work`, the features `link-screen-work` and `edit-screen-work`, the two existing widgets, two new views and routes, and a new navigation tab. `apps/web` is unchanged.
- devops: none (no migration, no new config).
- docs: ROADMAP (F3 done), Swagger.
