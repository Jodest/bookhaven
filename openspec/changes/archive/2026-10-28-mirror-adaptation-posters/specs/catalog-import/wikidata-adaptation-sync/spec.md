## MODIFIED Requirements

### Requirement: Adaptation data
For each adaptation the sync SHALL take:
- **Title.** The Russian label, else the English label, else the original title (P1476). An item without any of them is not linked and does not count in `found`.
- **Original title.** The original title (P1476) when it differs from the stored title.
- **Year.** The year of the earliest publication date (P577). A year outside 1880 to 2100 is ignored.
- **Directors.** Up to three director names (P57), each by its Russian label, else its English label, sorted by name and joined with «, ».
- **IMDb id.** The first IMDb id (P345) of the form `tt` followed by digits.
- **Kinopoisk id.** The first Kinopoisk film id (P2603) made only of digits.
- **Poster hints.** One film poster file (P3383), one image file (P18), one TMDB movie id (P4947) and one TMDB TV series id (P4983). The TMDB ids are digits only. They are only used to find a poster (see «Adaptation posters»). Only the one TMDB id whose poster the work actually gets is kept, together with that poster, so that the poster can be re-checked later (see `screen-works/poster-copies`). It is not shown anywhere.

A missing or malformed value leaves that field empty without failing the item.

#### Scenario: Several release dates
- **WHEN** an adaptation has the publication dates 1967 and 1972
- **THEN** its year is 1967 and it is linked once

#### Scenario: No Russian label
- **WHEN** an adaptation has only an English label «The Master and Margarita»
- **THEN** it is linked with that English title

#### Scenario: Malformed TMDB id
- **WHEN** an adaptation's TMDB movie id is not made of digits
- **THEN** it is linked as usual and TMDB is not asked about it

### Requirement: Adaptation posters
For each adaptation Wikidata lists for a synced book, the sync SHALL fill the work's poster only when all of these hold:
- the work has no poster;
- an admin did not remove its automatic poster;
- the work was not edited by hand.

It MUST NOT replace or remove any existing poster, in particular one uploaded by an admin. The sources are tried in this order, and the first usable one wins:
1. **Wikimedia Commons.** The film poster file (P3383), then the image file (P18). A file is usable when:
   - its licence is public domain, CC0, CC BY or CC BY-SA and it is not marked non-free;
   - Commons returns a thumbnail on a Wikimedia upload host;
   - for CC BY and CC BY-SA, its author is known.

   The poster keeps the author, the licence, the licence URL and the file page. The Wikimedia thumbnail is the poster's source link. The image is copied to the site's own storage (see `screen-works/poster-copies`).
2. **TMDB**, only when a TMDB API key is configured. It takes the poster of the TMDB movie id (P4947) or of the TMDB TV series id (P4983); the TV id is tried first for series and animated series. The poster is requested for the Russian language. The poster's source link is on TMDB's image server at width 500. The image is copied to the site's own storage, and the TMDB id it came from is kept for the re-check (see `screen-works/poster-copies`). Without a key this step is skipped silently, and TMDB is never called.
3. Otherwise the work stays without a poster, and a later sync tries again.

Every poster the sync fills queues a copy of it. A failure to queue does not change the sync's result. The sync covers works that are already linked, so re-running it fills the posters of existing adaptations. This applies to a single book, a bulk sync, the catalog sync job and the sync after a harvest. A Commons or TMDB failure MUST NOT change a book's outcome, its `added` or the HTTP status, and the affected work stays without a poster. One chunk of up to 50 books makes at most 100 TMDB lookups; the remaining works wait for a later sync. The film sync of authors does not fetch posters.

#### Scenario: Commons film poster
- **WHEN** a synced adaptation has a public-domain film poster file on Commons
- **THEN** the work gets a poster with `source: "commons"` whose source link is a Wikimedia thumbnail, a copy of it is queued, and its item counts it in `posters`

#### Scenario: Non-free image falls back to TMDB
- **WHEN** an adaptation's only Commons image is not free, it has a TMDB movie id and the TMDB key is configured
- **THEN** the work gets a poster with `source: "tmdb"` whose source link starts with `https://image.tmdb.org/t/p/w500/`

#### Scenario: No TMDB key
- **WHEN** the TMDB key is not configured and an adaptation has only a TMDB movie id
- **THEN** no TMDB request is made, the work has no poster and the book's outcome is unaffected

#### Scenario: Backfill on re-sync
- **WHEN** an admin syncs again a book whose three adaptations were linked before posters existed and Wikidata has posters for two of them
- **THEN** the outcome is `unchanged` with `added: 0` and `posters: 2`

#### Scenario: Uploaded poster kept
- **WHEN** a work has a poster uploaded by an admin and Wikidata has a free film poster for it
- **THEN** the sync keeps the uploaded poster

#### Scenario: Removed poster stays removed
- **WHEN** an admin removed the automatic poster of a work and the book is synced again
- **THEN** the work still has no poster

#### Scenario: Work edited by hand
- **WHEN** an admin fixed the title of a work without a poster and the book is synced again
- **THEN** the work still has no poster and keeps the admin's title

#### Scenario: TMDB unavailable
- **WHEN** every TMDB request of a sync fails
- **THEN** the books get their usual outcomes and links, and the works that needed TMDB stay without a poster
