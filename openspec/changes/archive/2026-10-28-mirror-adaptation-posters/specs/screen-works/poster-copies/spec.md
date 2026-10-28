## Purpose
Defines how BookHaven keeps its own copies of automatic screen work posters from Wikimedia Commons and TMDB, and which poster link the API returns. It also covers how TMDB posters are re-checked and purged to meet the TMDB API terms.

## ADDED Requirements

### Requirement: Poster link and poster copy
For a work whose poster source is `commons` or `tmdb`, the stored poster link SHALL remain the source of the poster. The system keeps at most one current copy per work, and the copy belongs to the exact link it was made from. Copies use the cover storage and follow its rules: download rules, retries and pacing, sweep, storage and cleanup, and serving (see `catalog/book-covers`). A copy is in one of these states:
- `pending`: not made yet for the current link, or a retry is due;
- `stored`: a file is kept;
- `failed`: the download was refused or gave up. A permanent failure is attempted again only when the poster link changes.

The displayed `poster.url`, in every public and admin response that has a `poster`, is:
- the site path `/covers/<shard>/<file>` when a copy of the current link is stored;
- the source link in every other case: no copy yet, pending or failed.

A poster uploaded by an admin (`manual`) is not copied. It is still served from `/posters/...`. When a work's poster link changes, or its poster is removed or replaced by an upload, a copy made from the old link MUST NOT be shown again. Its file is deleted once nothing refers to it. Attribution fields (`author`, `license`, `license_url`, `source_url`) and `source` are unaffected by copying.

#### Scenario: Copied poster
- **WHEN** a reader opens a book whose adaptation has a Commons poster with a stored copy
- **THEN** the item's `poster.url` is a `/covers/…` path on the site's own origin, `poster.source` is `commons` with the same attribution, and the browser makes no request to Wikimedia for it

#### Scenario: Not copied yet
- **WHEN** the sync has just given a work a TMDB poster and the worker has not copied it yet
- **THEN** the work's `poster.url` is the `https://image.tmdb.org/t/p/w500/…` link

#### Scenario: Worker down
- **WHEN** the worker is stopped and a work's poster has never been copied
- **THEN** the work keeps showing its source link and the book page still shows the poster

#### Scenario: Automatic poster removed
- **WHEN** an admin removes the copied Commons poster of a work
- **THEN** the work has `poster: null` at once, and the copied file is deleted by a later sweep when no book or work refers to it

#### Scenario: Upload replaces a copied poster
- **WHEN** an admin uploads a poster for a work whose TMDB poster was copied
- **THEN** the work's `poster.url` starts with `/posters/` and the TMDB copy is never shown again

### Requirement: Queuing a poster copy
Every adaptation sync that gives a work a `commons` or `tmdb` poster SHALL queue a copy of that work's poster once the poster is saved. This covers the admin sync of one or more books, the catalog sync job and the sync after a harvest. Queuing MUST NOT delay or fail the sync, change its outcomes or counters, or change the HTTP status. If queuing fails, the periodic sweep makes the copy later. The sweep also copies every existing `commons` or `tmdb` poster that has no copy of its current link, so posters that existed before copying was introduced are copied without a new sync. Works linked to authors share the same poster and copy.

#### Scenario: Sync fills a poster
- **WHEN** an admin syncs a book and one of its adaptations gets a Commons poster
- **THEN** a copy of that poster is queued, and after the worker stores it the adaptation's `poster.url` is a `/covers/…` path

#### Scenario: Existing posters backfilled
- **WHEN** the catalog has 30 works with Commons or TMDB posters and no copies
- **THEN** the sweeps copy them without any sync running, and each work shows its source link until its copy is stored

#### Scenario: Queue unavailable during a sync
- **WHEN** an admin syncs a book while the task queue is down and a work gets a poster
- **THEN** the sync response is unchanged, the work shows the source link, and a later sweep stores the copy

#### Scenario: Author film shares the copy
- **WHEN** a work with a stored poster copy is linked to a book and to an author
- **THEN** the author's film list returns the same `/covers/…` poster link

### Requirement: TMDB poster re-check
Every TMDB poster SHALL be re-checked against the TMDB API between 150 and 164 days after it was set or last confirmed. The exact day is spread at random, so that posters set together are not all re-checked at once. The re-check asks TMDB for the poster of the TMDB movie or TV id it came from, in Russian, at width 500:
- **Same poster.** The poster and its copy are kept, and the next re-check is scheduled the same way.
- **Changed poster.** The poster link is replaced with the new one, the new image is copied, and the old copy is no longer shown.
- **No poster, or unknown id.** The TMDB poster and its copy are dropped, and the work has no poster. A later sync MAY give it a poster again.
- **TMDB unavailable** (network error, rate limit, server error or rejected key). The poster is kept and the re-check is tried again about a day later.
- **No TMDB API key configured, or no TMDB id kept for the poster.** The TMDB poster and its copy are dropped.

A TMDB poster, with its link and its copy, MUST NOT be kept more than 175 days after it was set or last confirmed. When that limit is reached it is dropped whatever the reason. Commons posters and their copies are never re-checked. Uploaded posters and posters removed by an admin are not affected. Dropping a poster never marks the work as edited by hand and never sets `poster_removed`.

#### Scenario: Unchanged TMDB poster
- **WHEN** a TMDB poster set 155 days ago is due and TMDB returns the same poster
- **THEN** the work keeps its poster and copy, and its next re-check is between 150 and 164 days later

#### Scenario: TMDB poster changed
- **WHEN** a due re-check gets a different poster file from TMDB
- **THEN** the work's poster becomes the new TMDB link, it is copied, and the old copy is never shown again

#### Scenario: TMDB poster gone
- **WHEN** a due re-check finds that TMDB no longer has a poster for the id
- **THEN** the work has `poster: null` and `poster_removed: false`

#### Scenario: Key removed
- **WHEN** a TMDB poster is due for its re-check and no TMDB API key is configured
- **THEN** the poster and its copy are dropped, and the work shows the placeholder

#### Scenario: TMDB down for weeks
- **WHEN** every re-check of a TMDB poster fails with server errors until 175 days have passed since it was confirmed
- **THEN** the poster and its copy are dropped at that point

#### Scenario: Commons copy kept
- **WHEN** a Commons poster was copied 300 days ago
- **THEN** it is still shown from its copy and was never re-checked

### Requirement: TMDB purge
The deployment SHALL provide a maintenance command, run by the operator on the server, that removes every TMDB-sourced poster. It has no HTTP endpoint, so only someone with shell access to the server can run it. For every work whose poster source is `tmdb`:
- the poster link, its kept TMDB id and its copy are removed, and the work has no poster;
- every copied file that no book or work refers to any more is deleted at once.

Commons posters, uploaded posters and posters removed by an admin are left unchanged. The command prints the number of posters removed and files deleted. A dry run prints the same numbers without changing anything. When a TMDB API key is still configured, the command warns that a later sync can fill TMDB posters again.

#### Scenario: Licence ended
- **WHEN** the operator runs the purge on a catalog with 12 TMDB posters, 8 of them copied, and 5 Commons posters
- **THEN** no work has a TMDB poster any more, the 8 copied files are deleted, and the 5 Commons posters are unchanged

#### Scenario: Dry run
- **WHEN** the operator runs the purge as a dry run
- **THEN** it reports how many posters and files would be removed and nothing changes

#### Scenario: No HTTP access
- **WHEN** an admin looks for a way to purge TMDB posters through the API or the admin app
- **THEN** there is none, and the purge is available only as the server command
