# screen-works/author-films Specification

## Purpose
Shows readers the films made about an author, both fiction biopics and documentaries, on the public author page. Lets admins hide a wrong film link without deleting it.

## Requirements

### Requirement: Public author films endpoint
`GET /api/v1/authors/{id}/screen-works` SHALL be open to everyone without a token. `{id}` is the author's id, not the slug. It returns 200 with `{items, total}`, the author's visible films without pagination. Each item is `{id, title, original_title, kind, relation, year, director, imdb_url, kinopoisk_url}`:
- `kind` is one of `film`, `tv_film`, `series`, `animated_film`, `animated_series` and `teleplay`.
- `relation` is `biopic` for a fiction film about the author and `documentary` for a documentary.
- Fields without a value are `null`.
- `imdb_url` is `https://www.imdb.com/title/<imdb id>/` and `kinopoisk_url` is `https://www.kinopoisk.ru/film/<kinopoisk id>/`.

Items are ordered by year, oldest first, with works without a year last, then by title. A link hidden by an admin MUST NOT be returned. A malformed id returns 400 «invalid author id». An unknown author, or an author that is not published, returns 404 «author not found».

#### Scenario: Visible films
- **WHEN** a published author has a biopic from 2019, a documentary from 2011 and a hidden film linked
- **THEN** the response has `total: 2`, with the 2011 documentary first with `relation: "documentary"`, and the hidden film is absent

#### Scenario: Pending author
- **WHEN** anyone requests the films of a pending author
- **THEN** the response is 404

#### Scenario: Author without films
- **WHEN** a published author has no visible film
- **THEN** the response is 200 with `items: []` and `total: 0`

#### Scenario: Slug instead of id
- **WHEN** anyone requests `/api/v1/authors/tolkien/screen-works`
- **THEN** the response is 400 «invalid author id»

### Requirement: Films block on the author page
The public author page SHALL show a block «Фильмы об авторе» when the author has at least one visible film. Each film shows:
- its title;
- its year and its type, shown as «Фильм», «Телефильм», «Сериал», «Мультфильм», «Мультсериал» or «Телеспектакль»;
- the mark «Документальный» when it is a documentary;
- «Режиссёр: <names>» when directors are known;
- the links «Кинопоиск» and «IMDb» for the ids it has, opening in a new tab.

No poster is shown. When there is no visible film, or the films cannot be loaded, the block MUST NOT be rendered and the rest of the page is unaffected.

#### Scenario: Author with a biopic and a documentary
- **WHEN** a reader opens a published author with a visible biopic that has a Kinopoisk id and an IMDb id, and a visible documentary with neither
- **THEN** the block «Фильмы об авторе» lists both, only the documentary carries «Документальный», the biopic has the links «Кинопоиск» and «IMDb» and the documentary has no links

#### Scenario: Nothing to show
- **WHEN** a reader opens an author whose only film is hidden
- **THEN** the page has no «Фильмы об авторе» block

#### Scenario: Films fail to load
- **WHEN** the films request fails while a reader opens an author page
- **THEN** the page shows the author and the author's books without a «Фильмы об авторе» block

### Requirement: Admin author film links
`GET /api/v1/admin/authors/{id}/screen-works` SHALL require the `admin` role. It returns every linked film of an author of any status, including hidden ones, in the public order. Each item has the public fields plus `hidden`, `source` (`wikidata` or `manual`), `qid` (the work's Wikidata QID or `null`), `imdb_id` and `kinopoisk_id`. An unknown author returns 404.

`PATCH /api/v1/admin/authors/{id}/screen-works/{screen_work_id}` with `{"hidden": true|false}` SHALL require the `admin` role and a bearer header. It sets the link's visibility and returns 200 with the updated item. A missing `hidden` or a malformed id returns 400. An unknown author or a link that does not exist returns 404 «author film link not found».

Hiding never deletes the link or the work, so a later sync cannot bring it back (see `catalog-import/wikidata-author-film-sync`). Hiding a film for an author MUST NOT change the visibility of the same work as a book adaptation. A moderator gets 403 on both endpoints.

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

### Requirement: Films section on the author edit page
The admin author edit page SHALL show a section «Фильмы об авторе». It lists every linked film with its title, year, type, directors and «Документальный» for documentaries. Hidden films carry the mark «Скрыт». Each row has «Скрыть», or «Показать» for a hidden one, which applies at once and refreshes the list. The section offers «Найти фильмы об авторе в Wikidata» when the author has a Wikidata QID. It shows:
- «Фильмов об авторе пока нет.» with no links;
- «Не удалось загрузить фильмы об авторе.» when the list fails to load;
- «Не удалось изменить видимость фильма.» when hiding or showing fails.

#### Scenario: Hiding from the edit page
- **WHEN** an admin clicks «Скрыть» on a film in the section
- **THEN** the row shows «Скрыт» and the button becomes «Показать»

#### Scenario: Author without a QID
- **WHEN** an admin opens the edit page of an author without a Wikidata QID
- **THEN** the section has no «Найти фильмы об авторе в Wikidata» button
