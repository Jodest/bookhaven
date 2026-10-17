## MODIFIED Requirements

### Requirement: Rating widget
The book page SHALL show signed-in readers a 1–5 star control (each star labelled «Оценить на N») with «Моя оценка: N» once rated and «Оцените книгу» before (the label sits on its own line above large stars); hovering a star previews the rating up to that star; on wide screens the label and the stars are centred in the actions column; changing it saves immediately, and a failure shows «Ошибка сохранения оценки».
- **Placement.** The control sits among the book actions under the cover (see `catalog/book-page`).
- **Summary refresh.** After a successful change, the page's rating summary MUST show the book's updated average and count without a manual reload.
- **Not signed in.** A visitor who is not signed in sees «Войдите, чтобы оценить», which leads to sign-in and back to the book page.
- **Hydration.** The control MUST render the same markup on the server and in the browser, so the page loads without a hydration error.

#### Scenario: Rating from the book page
- **WHEN** a signed-in reader clicks the fourth star
- **THEN** the rating 4 is saved and shown as «Моя оценка: 4»

#### Scenario: Summary follows my rating
- **WHEN** a signed-in reader gives the first rating, a 5, to a book without ratings
- **THEN** the rating summary changes from «Пока нет оценок» to «5.0 · 1 оценка»

#### Scenario: Rating fails
- **WHEN** saving a rating fails
- **THEN** the previous own rating is shown again with «Ошибка сохранения оценки»
