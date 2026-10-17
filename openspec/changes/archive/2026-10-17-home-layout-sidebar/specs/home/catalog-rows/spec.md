## MODIFIED Requirements

### Requirement: New books
«Новое в каталоге» SHALL hold up to 10 published books in the order defined by `catalog-moderation/entity-status` (first-publication time, newest first); the page shows the first ones that fit in one row. Its link «Весь каталог» opens `/books?sort=new`, which sorts by publication year instead (see `catalog-moderation/entity-status`).

#### Scenario: Row size
- **WHEN** 25 books are published
- **THEN** `new_books` in the home payload holds 10 of them
