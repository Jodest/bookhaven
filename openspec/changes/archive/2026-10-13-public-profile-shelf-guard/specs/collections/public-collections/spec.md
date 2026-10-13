## MODIFIED Requirements

### Requirement: Public pages
The web app SHALL provide `/shelves/public/{id}` («Полка», «На этой полке пока нет книг», «Полка не найдена») and `/lists/public/{id}` («Публичный список», «В этом списке пока нет книг», «Список не найден или недоступен»), linked from the public profile. Both pages MUST open for everyone, signed in or not (see `auth/authorization`).

#### Scenario: Anonymous visitor follows a public list link
- **WHEN** a visitor who is not signed in opens `/lists/public/{id}` of a public list
- **THEN** the list is shown

#### Scenario: Anonymous visitor follows a public shelf link
- **WHEN** a visitor without session cookies opens `/shelves/public/{id}`
- **THEN** the shelf is shown without a redirect to login
