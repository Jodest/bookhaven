## MODIFIED Requirements

### Requirement: Public adaptations endpoint
`GET /api/v1/books/{id}/adaptations` SHALL be open to everyone without a token. It returns 200 with `{items, total}`, the book's visible adaptations without pagination. Each item is `{id, title, original_title, kind, year, director, imdb_url, kinopoisk_url, poster}`:
- `kind` is one of `film`, `tv_film`, `series`, `animated_film`, `animated_series` and `teleplay`.
- Fields without a value are `null`.
- `imdb_url` is `https://www.imdb.com/title/<imdb id>/` and `kinopoisk_url` is `https://www.kinopoisk.ru/film/<kinopoisk id>/`.
- `poster` is `null` for a work without a poster, else `{url, source, author, license, license_url, source_url}`:
  - `source` is `manual` for a poster uploaded by an admin, `commons` for a Wikimedia Commons image and `tmdb` for a TMDB poster.
  - `url` is the site path `/posters/<shard>/<file>` for `manual`. For `commons` and `tmdb` it is the site path `/covers/<shard>/<file>` once a copy of the current poster link is stored (see `screen-works/poster-copies`). Before that it is the source link: an `https://upload.wikimedia.org/...` or `https://thumb.wikimedia.org/...` thumbnail for `commons`, and `https://image.tmdb.org/t/p/w500/<file>` for `tmdb`.
  - `author`, `license`, `license_url` and `source_url` (the Commons file page) are set only for `commons`, each `null` when unknown, and are `null` for the other sources.

Items are ordered by year, oldest first, with works without a year last, then by title. A link hidden by an admin MUST NOT be returned. A malformed id returns 400 «invalid book id». An unknown book, or a book that is not published, returns 404 «book not found».

#### Scenario: Visible adaptations
- **WHEN** a published book has a film from 1994 linked, a series from 1972 linked and a hidden film linked
- **THEN** the response has `total: 2`, with the 1972 series first, and the hidden film is absent

#### Scenario: Pending book
- **WHEN** anyone requests the adaptations of a pending book
- **THEN** the response is 404

#### Scenario: Book without adaptations
- **WHEN** a published book has no visible adaptation
- **THEN** the response is 200 with `items: []` and `total: 0`

#### Scenario: Poster of an adaptation
- **WHEN** a published book has one adaptation with an uploaded poster and one without any poster
- **THEN** the first item has `poster.source: "manual"` and a `poster.url` starting with `/posters/`, and the second item has `poster: null`

#### Scenario: Commons poster attribution
- **WHEN** an adaptation's poster comes from a CC BY-SA file on Wikimedia Commons
- **THEN** its `poster` has `source: "commons"`, the file's author and licence, the licence URL and the Commons file page in `source_url`

#### Scenario: Copied TMDB poster
- **WHEN** an adaptation's TMDB poster has a stored copy
- **THEN** its `poster` has `source: "tmdb"` and a `url` starting with `/covers/`

### Requirement: Poster attribution
Wherever a poster is shown on the public site, a Wikimedia Commons poster SHALL be credited under its image as «Постер: <author>, <license>, Wikimedia Commons». The author part is left out when unknown, and the licence links to its address when known. «Wikimedia Commons» links to the file page. Both links open in a new tab. Uploaded and TMDB posters have no caption. Copied posters are credited exactly like posters shown from their source.

The public site footer SHALL always show, in this order:
- «Постеры фильмов — Wikimedia Commons и TMDB.»;
- the official TMDB logo. It links to `https://www.themoviedb.org`, opens in a new tab and has the accessible name «TMDB». It MUST be shown smaller and less prominent than the BookHaven logo of the site header;
- the notice «BookHaven uses TMDB and the TMDB APIs but is not endorsed, certified, or otherwise approved by TMDB.»

#### Scenario: Commons poster caption
- **WHEN** an adaptation's poster is a Commons file by «Jane Doe» under «CC BY-SA 4.0»
- **THEN** the card shows «Постер: Jane Doe, CC BY-SA 4.0, Wikimedia Commons» with links to the licence and the file page

#### Scenario: TMDB notice
- **WHEN** a reader opens any page of the public site
- **THEN** the footer contains «BookHaven uses TMDB and the TMDB APIs but is not endorsed, certified, or otherwise approved by TMDB.» and no longer contains «This product uses the TMDB API but is not endorsed or certified by TMDB.»

#### Scenario: TMDB logo
- **WHEN** a reader looks at the footer of any page
- **THEN** it shows the TMDB logo as a link named «TMDB» to `https://www.themoviedb.org`, smaller than the BookHaven logo in the header
