# catalog-import/wikidata-matching Specification

## Purpose

Rules for matching an author or genre coming from Wikidata (a display name plus an optional QID) to the existing catalog, shared by the book harvest and the direct author/genre imports, so these flows never create duplicates and never revive an entity an admin rejected.

## Requirements

### Requirement: Author matching by QID first
When a Wikidata author reference carries a QID, the system SHALL first look for an author with that Wikidata QID. A rejected author found this way MUST act as a tombstone: nothing is linked and nothing is created. A pending or published author found this way MUST be reused unchanged.

#### Scenario: QID matches a published author
- **WHEN** a reference with QID `Q7200` arrives and a published author already has QID `Q7200`
- **THEN** that author is reused and no author is created

#### Scenario: QID matches a rejected author
- **WHEN** a reference with QID `Q7200` arrives and a rejected author has QID `Q7200`
- **THEN** the reference is skipped (harvest) or reported as `rejected` (direct import), and no author is created

### Requirement: Author matching by name slug
When there is no QID match, the system SHALL match an author by the normalized name slug. On a slug match the system MUST apply these outcomes:
- the matched author has no QID and a QID was given: the QID is backfilled, the status is unchanged, and the author is reused;
- the matched author is rejected: it stays a tombstone (the QID is still backfilled if it had none);
- the matched author has the same QID, or no QID was given: the author is reused;
- the matched author has a different QID (a namesake): a new pending author is created with the slug `<name-slug>-q<id>` and the existing author is left untouched.

#### Scenario: Slug match without a QID backfills it
- **WHEN** a reference «Лев Толстой» with QID `Q7200` arrives and an author with the same slug has no QID
- **THEN** QID `Q7200` is stored on that author, its status does not change, and it is reused

#### Scenario: Namesake with a different QID
- **WHEN** a reference with QID `Q171976` matches by slug an author whose QID is `Q7200`
- **THEN** a new pending author with slug suffix `-q171976` is created and the existing author is not modified

### Requirement: Unmatched authors are created pending
When neither the QID nor the name slug matches, the system SHALL create a new author with status `pending`, carrying the QID when one was given.

#### Scenario: New author from Wikidata
- **WHEN** a reference with a name and QID matches nothing in the catalog
- **THEN** a pending author with that name and QID is created

### Requirement: Genre matching by QID first, aliases followed
When a Wikidata genre reference carries a QID, the system SHALL first look for a genre with that QID, including merged (alias) genres. A matched alias MUST be followed one hop to its target genre, and the target's status decides the outcome. A rejected genre (direct or as an alias target) MUST act as a tombstone. A pending or published genre MUST be reused. Neither an alias nor its target is written when an alias is followed.

#### Scenario: QID belongs to a merged genre
- **WHEN** a reference's QID belongs to a genre that was merged into «фэнтези»
- **THEN** «фэнтези» is reused and nothing is created or modified

#### Scenario: QID matches a rejected genre
- **WHEN** a reference's QID matches a rejected genre
- **THEN** the reference is skipped (harvest) or reported as `rejected` (direct import)

### Requirement: Genre matching by name slug
When there is no QID match, the system SHALL match a genre by the normalized name slug, following an alias to its target the same way as the QID match. On a direct slug match:
- the genre has no QID and a QID was given: the QID is backfilled and the status is unchanged; a rejected genre then stays a tombstone;
- the genre has a different QID: the existing genre is reused untouched and no second genre is created;
- the genre has the same QID, or no QID was given: the genre is reused.

#### Scenario: Slug match with a different QID
- **WHEN** a genre reference matches by slug a genre that already has a different QID
- **THEN** the existing genre is reused unchanged and no new genre is created

### Requirement: Unmatched genres are created pending
When neither the QID nor the name slug matches, the system SHALL create a new genre with status `pending`, carrying the QID when one was given.

#### Scenario: New genre from Wikidata
- **WHEN** a genre reference matches nothing in the catalog
- **THEN** a pending genre with that name and QID is created

### Requirement: Matching never edits existing entities beyond a QID backfill
Matching SHALL NOT change an existing author's or genre's name, slug or status. The only write it MAY make to an existing entity is the QID backfill described above.

#### Scenario: Re-importing an existing pending author
- **WHEN** an author reference matches an existing pending author by QID
- **THEN** the author's name, slug and status stay exactly as they were
