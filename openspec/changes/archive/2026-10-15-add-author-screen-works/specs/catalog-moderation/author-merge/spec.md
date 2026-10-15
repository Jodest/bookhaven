## ADDED Requirements

### Requirement: Films about the author move with the merge
In the merge transaction, every film link of the duplicate SHALL be moved to the canonical author, with its relation, its source and its hidden flag. When both authors are linked to the same film, the canonical author keeps one link, and that link is hidden if either link was hidden. A hide MUST never be lost by a merge, so a later sync of the canonical author cannot bring back a film an admin hid on the duplicate. `books_reassigned` does not count film links.

#### Scenario: Duplicate with a hidden film
- **WHEN** the duplicate has a hidden link to a film the canonical author does not have, and the authors are merged
- **THEN** the canonical author has a hidden link to that film, and syncing the canonical author keeps it hidden

#### Scenario: Film linked to both
- **WHEN** both authors are linked to the same film, visible on the canonical author and hidden on the duplicate, and the authors are merged
- **THEN** the canonical author has one link to that film and it is hidden
