## MODIFIED Requirements

### Requirement: Public author films endpoint
`GET /api/v1/authors/{id}/screen-works` SHALL be open to everyone without a token. `{id}` is the author's id, not the slug. It returns 200 with `{items, total}`, the author's visible films without pagination. Each item is `{id, title, original_title, kind, relation, year, director, imdb_url, kinopoisk_url, poster}`:
- `kind` is one of `film`, `tv_film`, `series`, `animated_film`, `animated_series` and `teleplay`.
- `relation` is `biopic` for a fiction film about the author and `documentary` for a documentary.
- Fields without a value are `null`.
- `imdb_url` is `https://www.imdb.com/title/<imdb id>/` and `kinopoisk_url` is `https://www.kinopoisk.ru/film/<kinopoisk id>/`.
- `poster` is the work's poster with the same shape and rules as in the public adaptations endpoint (see `screen-works/book-adaptations`), or `null`. A work shared with a book adaptation has the same poster on both pages.

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

#### Scenario: Poster of a shared work
- **WHEN** a work with an uploaded poster is both a book adaptation and a film about the book's author
- **THEN** the author's item for that work has the same `poster` as the book's item

### Requirement: Films block on the author page
The public author page SHALL show a block «Фильмы об авторе» when the author has at least one visible film. The films are a grid of cards with the same poster frame, placeholder and Commons caption as the adaptation cards of the book page (see `screen-works/book-adaptations`). Each card shows:
- the poster in a 2:3 frame, or the placeholder with the type's icon and the year;
- its title;
- its year and its type, shown as «Фильм», «Телефильм», «Сериал», «Мультфильм», «Мультсериал» or «Телеспектакль»;
- the mark «Документальный» when it is a documentary;
- «Режиссёр: <names>» when directors are known;
- the links «Кинопоиск» and «IMDb» for the ids it has, opening in a new tab.

When there is no visible film, or the films cannot be loaded, the block MUST NOT be rendered and the rest of the page is unaffected.

#### Scenario: Author with a biopic and a documentary
- **WHEN** a reader opens a published author with a visible biopic that has a Kinopoisk id and an IMDb id, and a visible documentary with neither
- **THEN** the block «Фильмы об авторе» lists both, only the documentary carries «Документальный», the biopic has the links «Кинопоиск» and «IMDb» and the documentary has no links

#### Scenario: Nothing to show
- **WHEN** a reader opens an author whose only film is hidden
- **THEN** the page has no «Фильмы об авторе» block

#### Scenario: Films fail to load
- **WHEN** the films request fails while a reader opens an author page
- **THEN** the page shows the author and the author's books without a «Фильмы об авторе» block

#### Scenario: Film without a poster
- **WHEN** a reader opens an author with a visible documentary from 2011 that has no poster
- **THEN** its card shows the placeholder with the type icon and «2011»
