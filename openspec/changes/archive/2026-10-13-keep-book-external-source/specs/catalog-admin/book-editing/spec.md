## MODIFIED Requirements

### Requirement: External source on update
An update SHALL keep the book's stored external source and id: they are set only when a harvest creates the book, and no admin edit, status change or other update may change or clear them. A harvested book therefore keeps the «Импортировано» mark and stays recognised as already harvested after any number of admin edits.

#### Scenario: Editing a harvested book
- **WHEN** an admin fixes the description of a book that came from a Wikidata harvest and saves
- **THEN** the book keeps its external source and id, and a later harvest of the same work skips it
