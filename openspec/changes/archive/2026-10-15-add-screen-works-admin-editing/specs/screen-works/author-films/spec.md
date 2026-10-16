## MODIFIED Requirements

### Requirement: Admin author film links
`GET /api/v1/admin/authors/{id}/screen-works` SHALL require the `admin` role. It returns every linked film of an author of any status, including hidden ones, in the public order. Each item has the public fields plus `hidden`, `source` (`wikidata` or `manual`), `qid` (the work's Wikidata QID or `null`), `imdb_id` and `kinopoisk_id`. An unknown author returns 404.

`PATCH /api/v1/admin/authors/{id}/screen-works/{screen_work_id}` SHALL require the `admin` role and a bearer header. The body has `{"hidden": true|false}`, `{"relation": "biopic"|"documentary"}` or both. It sets the link's visibility, its relation or both, for a link of either source, and returns 200 with the updated item.
- A body with neither field returns 400 «hidden or relation is required».
- Any other `relation` returns 400 «invalid relation».
- A malformed id returns 400.
- An unknown author or a link that does not exist returns 404 «author film link not found».

Hiding never deletes the link or the work, so a later sync cannot bring it back (see `catalog-import/wikidata-author-film-sync`). A relation set by an admin stays, because the sync never changes the relation of an existing link. Hiding a film for an author MUST NOT change the visibility of the same work as a book adaptation. A moderator gets 403 on both endpoints.

#### Scenario: Hide a film
- **WHEN** an admin sends `{"hidden": true}` for a linked film of a published author
- **THEN** the response is 200 with `hidden: true` and the public endpoint no longer returns that film

#### Scenario: Show again
- **WHEN** an admin sends `{"hidden": false}` for a hidden film link
- **THEN** the film is visible on the public author page again

#### Scenario: Unknown link
- **WHEN** an admin patches a film that is not linked to the author
- **THEN** the response is 404

#### Scenario: Same work as a book adaptation
- **WHEN** a work is both a visible adaptation of a book and a film about that book's author, and an admin hides it for the author
- **THEN** the book page still shows it under «Экранизации»

#### Scenario: Moderator
- **WHEN** a moderator requests the admin film list of an author
- **THEN** the response is 403

#### Scenario: Fix the relation
- **WHEN** an admin sends `{"relation": "documentary"}` for a Wikidata film linked as `biopic`, and the author is synced again later
- **THEN** the response is 200 with `relation: "documentary"`, the public author page marks the film «Документальный», and after the sync the link is still `documentary`

#### Scenario: Empty body
- **WHEN** an admin sends `{}` for a linked film
- **THEN** the response is 400 «hidden or relation is required» and the link is unchanged

### Requirement: Films section on the author edit page
The admin author edit page SHALL show a section «Фильмы об авторе». It lists every linked film with its title, year, type, directors and «Документальный» for documentaries. Hidden films carry the mark «Скрыт», and manual links carry the mark «Вручную». Each row has:
- «Скрыть», or «Показать» for a hidden one, which applies at once and refreshes the list;
- a «Тип связи» switch, «Биографический» or «Документальный», which applies at once;
- «Изменить», which opens the work's edit page and returns to the author afterwards;
- for a manual link only, «Удалить», which asks «Удалить связь с «<title>»? Сам фильм останется в каталоге.» and then deletes the link and refreshes the list.

The section always offers «Добавить фильм об авторе» (see `screen-works/admin-editing`), and offers «Найти фильмы об авторе в Wikidata» when the author has a Wikidata QID. It shows:
- «Фильмов об авторе пока нет.» with no links;
- «Не удалось загрузить фильмы об авторе.» when the list fails to load;
- «Не удалось изменить видимость фильма.» when hiding or showing fails;
- «Не удалось изменить тип связи.» when changing the relation fails;
- «Не удалось удалить связь.» when deleting fails.

#### Scenario: Hiding from the edit page
- **WHEN** an admin clicks «Скрыть» on a film in the section
- **THEN** the row shows «Скрыт» and the button becomes «Показать»

#### Scenario: Author without a QID
- **WHEN** an admin opens the edit page of an author without a Wikidata QID
- **THEN** the section has no «Найти фильмы об авторе в Wikidata» button and has «Добавить фильм об авторе»

#### Scenario: Switch to documentary
- **WHEN** an admin switches «Тип связи» of a biopic row to «Документальный»
- **THEN** the row shows «Документальный» after the refresh
