## MODIFIED Requirements

### Requirement: Films block on the author page
The public author page SHALL show a block «Фильмы об авторе» when the author has at least one visible film. The films are shown as the same compact horizontal cards as the adaptations on the book page, with the same thumbnail, placeholder and Commons caption (see `screen-works/book-adaptations`). They are in one column on narrow screens and at least two columns on wide screens. Each card shows:
- on the left, the poster thumbnail in a 2:3 frame about 72 px wide, or the placeholder of the same size with the type's icon and the year;
- on the right, its title, then one wrapping line with:
  - its year and its type, shown as «Фильм», «Телефильм», «Сериал», «Мультфильм», «Мультсериал» or «Телеспектакль»;
  - the mark «Документальный» when it is a documentary;
  - «Режиссёр: <names>» when directors are known;
  - the links «Кинопоиск» and «IMDb» for the ids it has, opening in a new tab;
- then the compact Commons caption, when the poster has one.

All visible films are shown at once, without a «Показать все» button. When there is no visible film, or the films cannot be loaded, the block MUST NOT be rendered and the rest of the page is unaffected.

#### Scenario: Author with a biopic and a documentary
- **WHEN** a reader opens a published author with a visible biopic that has a Kinopoisk id and an IMDb id, and a visible documentary with neither
- **THEN** the block «Фильмы об авторе» lists both as compact cards. Only the documentary carries «Документальный». The biopic has the links «Кинопоиск» and «IMDb», and the documentary has no links.

#### Scenario: Nothing to show
- **WHEN** a reader opens an author whose only film is hidden
- **THEN** the page has no «Фильмы об авторе» block

#### Scenario: Films fail to load
- **WHEN** the films request fails while a reader opens an author page
- **THEN** the page shows the author and the author's books without a «Фильмы об авторе» block

#### Scenario: Film without a poster
- **WHEN** a reader opens an author with a visible documentary from 2011 that has no poster
- **THEN** its card shows the thumbnail-sized placeholder with the type icon and «2011»

#### Scenario: Many films
- **WHEN** a reader opens an author with six visible films
- **THEN** all six cards are visible and there is no «Показать все» button
