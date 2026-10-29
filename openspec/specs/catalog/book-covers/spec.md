# catalog/book-covers Specification

## Purpose
Defines how BookHaven keeps its own copies of book covers. It covers how the copies are downloaded safely, stored, served and kept in step with each book's cover link, and which cover link the API returns.

## Requirements

### Requirement: Cover link and cover copy
The cover link stored on a book SHALL remain the source of its cover. The system keeps at most one current copy per book, and the copy belongs to the exact link it was made from. A copy is in one of these states:
- `pending`: not made yet for the current link, or a retry is due;
- `stored`: a file is kept;
- `failed`: the download was refused or gave up;
- `absent`: the source says there is no cover;
- `linked`: the link is on a link-only host (see "Link-only cover hosts"), so no copy is made and the link is shown as it is.

When the book's link changes, a copy made from the old link MUST NOT be shown again.

#### Scenario: Link changed after copying
- **WHEN** a book has a stored copy and an admin changes its cover link to another image
- **THEN** until the new image is copied, the book shows the new link, never the old copy

#### Scenario: Link removed
- **WHEN** an admin clears the cover link of a book that had a stored copy
- **THEN** the book has no cover and shows «Нет обложки» on the site

#### Scenario: Stored copy replaced by a FantLab link
- **WHEN** an admin changes the cover link of a book with a stored copy to `https://fantlab.ru/images/editions/big/16469`
- **THEN** the copy becomes `linked`, the book shows the FantLab link, and the old copy is never shown again

### Requirement: Displayed cover link
Every API response that shows a book's cover SHALL return the displayed link and not the raw source. This covers the public book, catalog, shelf, home, review and collection responses, the `cover_urls` of collections, and the admin collection screens. The displayed link is:
- the relative path `/covers/{file}` when a copy of the current link is stored;
- `null` when the copy is `absent`;
- the source link in every other case: no copy yet, pending, failed or linked.

A book with no cover link has `null`. The admin book record is the only exception (see `catalog-admin/book-editing`). A book whose copy is `absent` MUST count as a book without a cover everywhere a cover matters, for example the book-of-the-day pools. A book whose copy is `linked` counts as a book with a cover.

#### Scenario: Copied cover
- **WHEN** a visitor opens a book whose current cover link has a stored copy
- **THEN** the book's `cover_url` is a `/covers/…` path on the site's own origin, and no request goes to Open Library

#### Scenario: Not copied yet
- **WHEN** a book was just saved with an Open Library cover link and no copy exists yet
- **THEN** the book's `cover_url` is the Open Library link

#### Scenario: Source has no cover
- **WHEN** Open Library answers that the book's cover link has no image
- **THEN** the book's `cover_url` is `null` and the book is not in the «books with a cover» pools of the book of the day

#### Scenario: Linked cover
- **WHEN** a visitor opens a book whose cover link is `https://fantlab.ru/images/editions/big/16469`
- **THEN** the book's `cover_url` is that FantLab link, and the book is in the «books with a cover» pools of the book of the day

### Requirement: Queuing a copy
Every save that writes a non-empty cover link SHALL queue a copy of that book once the save is committed. This covers:
- an admin creating or updating a book;
- each row of a CSV/JSON book import;
- a book created by a Wikidata harvest, including a cover filled in from Open Library by ISBN, by Open Library ID or by the title search, or from a FantLab edition;
- a cover link set by the cover backfill (see `catalog-import/wikidata-book-harvest`).

An admin update that keeps the same link queues a copy only when no copy of it is stored. A copy queued by a save is attempted even after an earlier failure of that link. Queuing MUST NOT delay or fail the save: if it fails, the save still succeeds and the periodic sweep makes the copy later. For a link on a link-only host, the queued copy only records the `linked` state and downloads nothing.

#### Scenario: File import
- **WHEN** an admin imports a CSV file whose three created rows have cover links
- **THEN** a copy is queued for each of the three books after its row is saved

#### Scenario: Queue unavailable
- **WHEN** an admin saves a book with a cover link while the task queue is down
- **THEN** the save succeeds, the book shows the source link, and a later sweep stores the copy

#### Scenario: Re-saving a failed cover
- **WHEN** an admin saves a book again, unchanged, after its cover copy failed
- **THEN** the copy is attempted again

#### Scenario: Backfilled cover
- **WHEN** the cover backfill sets the Open Library cover link of a book that had none
- **THEN** a copy is queued for that book, the book shows the Open Library link until the copy is stored, and a `/covers/…` path afterwards

#### Scenario: Harvested FantLab cover
- **WHEN** a harvest creates a book with the cover link `https://fantlab.ru/images/editions/big/16469`
- **THEN** a copy is queued, the copy becomes `linked` without any request to fantlab.ru, and the book keeps showing the FantLab link

### Requirement: Download rules
The download of a cover link, or of a screen work poster link (see `screen-works/poster-copies`), SHALL follow all of these rules:
- **Hosts.** The link MUST use `http` or `https` on port 80 or 443, and its host MUST be on the allowlist. The default allowlist is `covers.openlibrary.org`, `archive.org` with its subdomains, `upload.wikimedia.org` and `image.tmdb.org`; it is configurable.
- **Addresses.** Every address the host resolves to MUST be checked when connecting. Loopback, private, link-local, unique-local, multicast, unspecified and other non-public addresses are refused.
- **Redirects.** At most 5 redirects. Each target MUST pass the host and address rules again.
- **Limits.** The whole download is limited to 30 seconds and 5 MB.
- **Type.** The bytes MUST be a JPEG, PNG or WebP image. The type the server declares is ignored.
- **Open Library.** An Open Library cover link is requested with `default=false`. A 404 answer marks the copy `absent`.

A link that breaks a rule is not stored, and its copy becomes `failed`.

#### Scenario: Redirect to a private address
- **WHEN** an allowlisted cover link redirects to a host that resolves to `10.0.0.5`
- **THEN** no connection is made to that address and the copy is `failed`

#### Scenario: Host not allowed
- **WHEN** an admin saves a cover link on a host outside the allowlist
- **THEN** nothing is downloaded, the copy is `failed`, and the book shows the source link

#### Scenario: File too large
- **WHEN** the image behind a cover link is larger than 5 MB
- **THEN** the download stops at the limit and the copy is `failed`

#### Scenario: Not an image
- **WHEN** a cover link returns an HTML page or a GIF with an image content type
- **THEN** the copy is `failed` and nothing is stored

#### Scenario: TMDB poster host allowed by default
- **WHEN** a screen work has a poster link on `image.tmdb.org` and the allowlist is not configured
- **THEN** the poster is downloaded and stored like a cover

### Requirement: Retries and pacing
A transient failure (a timeout, a network error, HTTP 408, 429 or 5xx) SHALL be retried with backoff. The first retry comes after 10 minutes, and each next pause doubles, up to 24 hours. After 6 failed attempts the copy is `failed`, and it is tried once more after 7 days. Other failures are permanent and MUST NOT be retried until the link changes or an admin saves the book again (for a poster: until its poster link changes): a host off the allowlist, a refused address, too many redirects, a file over the limit, a non-image, or any other 4xx. Downloads of covers and posters run one at a time, together, at least 1 second apart.

#### Scenario: Temporary outage
- **WHEN** Open Library answers 503 for a cover
- **THEN** the copy stays `pending` and is attempted again about 10 minutes later, and the book shows the source link meanwhile

#### Scenario: Poster outage
- **WHEN** Wikimedia answers 503 for a Commons poster
- **THEN** the poster copy stays `pending`, is attempted again about 10 minutes later, and the work shows the source link meanwhile

### Requirement: Periodic sweep
A periodic sweep SHALL run every 15 minutes by default. The schedule is configurable and the sweep can be turned off. Each run:
- copies up to 50 items in total (configurable), books and screen work posters together. These are books with a cover link and works with a Commons or TMDB poster that have no copy for their current link, or whose retry is due. The items whose copy has been waiting longest come first;
- checks a rolling batch of stored copies of books and posters, and marks any copy whose file is missing as `pending` so it is copied again;
- when the storage is new or empty while copies are recorded as stored, marks all of them, of books and posters, `pending` at once;
- re-checks TMDB posters that are due (see `screen-works/poster-copies`).

While a copy is not stored, the book or work shows its source link.

#### Scenario: Fresh database or empty storage
- **WHEN** the storage is empty and 24 books have Open Library cover links
- **THEN** the books show their source links until the sweeps have copied them, after which they show `/covers/…` paths

#### Scenario: Lost file
- **WHEN** the file of a stored copy has disappeared from the storage
- **THEN** a later sweep copies the cover again, and the book shows the source link in between

#### Scenario: Storage rebuilt with poster copies
- **WHEN** the cover storage is replaced by an empty one while 10 posters and 20 covers are recorded as stored
- **THEN** the first sweep marks all 30 copies `pending`, the books and works show their source links, and later sweeps copy them again

### Requirement: Storage and cleanup
Stored files SHALL be named by a hash of their content, so books and works with an identical image share one file. A file MUST be deleted once no book cover and no screen work poster refers to it. This happens either after a replacement copy is stored, or in the sweep, which removes files that nothing refers to and that are older than one hour. Copies are not backed up: they can always be made again from the cover and poster links.

#### Scenario: Cover replaced
- **WHEN** a book's new cover is stored and no other book uses its old file
- **THEN** the old file is deleted

#### Scenario: Shared file kept
- **WHEN** two books use the same image and one of them gets a new cover
- **THEN** the shared file stays for the other book

#### Scenario: Poster copy kept by cleanup
- **WHEN** the sweep cleans up the storage while a work's stored poster copy is older than one hour
- **THEN** the poster file is kept because the work refers to it

### Requirement: Serving copies
`GET /covers/{file}` and `HEAD /covers/{file}` SHALL be public: no sign-in, no bearer and no role. They are served by the API and, through a proxy, on the origins of the public site and the admin app. The response carries:
- the file with its image content type;
- `Cache-Control: public, max-age=31536000, immutable`;
- `X-Content-Type-Options: nosniff`;
- an `ETag`. A matching `If-None-Match` returns 304.

A name that is malformed or unknown returns 404. There is no directory listing, and any path that tries to leave the storage returns 404.

#### Scenario: Copy served
- **WHEN** a browser requests the `/covers/…` path of a stored cover
- **THEN** the response is 200 with the image type and `Cache-Control: public, max-age=31536000, immutable`

#### Scenario: Path traversal
- **WHEN** a client requests `/covers/../../etc/passwd` or `/covers/`
- **THEN** the response is 404 and nothing outside the stored covers is read

### Requirement: Link-only cover hosts
The system SHALL keep a list of link-only cover hosts. The default is `fantlab.ru` with its subdomains, and the list is configurable. Hosts are matched the same way as the download allowlist. A host on both lists counts as link-only. This list applies to book covers only. Screen work posters keep the download rules unchanged.

For a book whose current cover link is on a link-only host:
- the system MUST NOT request the link, neither for a copy nor for any check. No download, redirect or retry is made, and the copy does not use the 1-second download pacing;
- the copy is `linked`, with no error, no file and no attempts;
- the book shows its source link, as "Displayed cover link" requires;
- the sweep takes a `linked` copy again only once 7 days have passed since it was recorded. If the host is still link-only by then, it records `linked` again without any request. If the host has been taken off the list, the copy is handled like a `pending` one under the normal download rules;
- a file stored for an earlier link of the book is cleaned up under "Storage and cleanup".

The public site MUST show a cover on a link-only host as a direct link: the visitor's browser loads the image from that host. Neither the site's server nor its image optimizer fetches, resizes or caches it.

#### Scenario: FantLab link never downloaded
- **WHEN** a book gets the cover link `https://fantlab.ru/images/editions/big/16469` and its copy is processed
- **THEN** no request goes to fantlab.ru, the copy is `linked` with no error, and the book's `cover_url` is the FantLab link

#### Scenario: Weekly re-check without a request
- **WHEN** a sweep runs 8 days after a book's FantLab copy became `linked`, and `fantlab.ru` is still link-only
- **THEN** the copy stays `linked`, it is due again 7 days later, and no request goes to fantlab.ru

#### Scenario: Host taken off the list
- **WHEN** an operator removes `fantlab.ru` from the link-only list, adds it to the download allowlist, and the book's `linked` copy is due
- **THEN** the sweep downloads the FantLab image and stores a copy, and the book shows a `/covers/…` path afterwards

#### Scenario: Browser loads the FantLab cover
- **WHEN** a visitor opens a catalog page that shows a book with a FantLab cover link
- **THEN** the image request goes from the visitor's browser to fantlab.ru, not through the site's image optimizer
