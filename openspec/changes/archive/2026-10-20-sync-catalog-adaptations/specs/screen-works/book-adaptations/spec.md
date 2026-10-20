## ADDED Requirements

### Requirement: Adaptation filter in the catalog API
`GET /api/v1/books` SHALL accept the optional query parameter `has_adaptation`. With `has_adaptation=true` the list, its `total` and its pages MUST contain only books that have at least one visible adaptation link, that is a link an admin did not hide. The filter combines with every other catalog filter, the search and the sort. Any other value of the parameter, or its absence, MUST leave the list unfiltered. The endpoint stays open to everyone without a token, and still lists only published books.

#### Scenario: Only books with a visible adaptation
- **WHEN** anyone requests `/api/v1/books?has_adaptation=true` and the catalog has a published book with one visible adaptation, a published book whose only adaptation is hidden and a published book without adaptations
- **THEN** only the first book is returned and `total` is 1

#### Scenario: Combined with a genre filter
- **WHEN** anyone requests the books of one genre with `has_adaptation=true`
- **THEN** only books of that genre with a visible adaptation are returned

#### Scenario: Unknown value
- **WHEN** anyone requests `/api/v1/books?has_adaptation=yes`
- **THEN** the list is the same as without the parameter

### Requirement: «Есть экранизация» in the catalog filters
The public catalog filter panel SHALL offer the checkbox «Есть экранизация», on desktop and in the mobile filters. Checking it adds `has_adaptation=true` to the page URL, resets to the first page and shows only books with a visible adaptation. While it is on, the active filters show the chip «Есть экранизация», and removing the chip or unchecking the box removes the parameter. Opening a catalog URL with `has_adaptation=true` shows the checkbox checked and the chip.

#### Scenario: Reader turns the filter on
- **WHEN** a reader on page 3 of the catalog checks «Есть экранизация»
- **THEN** the URL has `has_adaptation=true` and no page beyond the first, the list shows only books with a visible adaptation, and the chip «Есть экранизация» is shown

#### Scenario: Shared link
- **WHEN** a reader opens a catalog link with `has_adaptation=true`
- **THEN** the checkbox «Есть экранизация» is checked and the chip is shown

#### Scenario: Removing the chip
- **WHEN** a reader removes the chip «Есть экранизация»
- **THEN** the parameter leaves the URL and the list is no longer filtered by adaptations
