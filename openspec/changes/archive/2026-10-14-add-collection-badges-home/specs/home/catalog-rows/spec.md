## ADDED Requirements

### Requirement: Curated collections row
«Подборки» SHALL show BookHaven's curated collections (see `collections/curated-collections`).
- **Which collections.** Take the 6 published collections that were first published most recently, in the order of the public collection index. Keep those with at least one public place, and show the first 3 of them.
- **Cards.** The cards are the same as on `/top`: the title, up to 4 covers, «{shown_count} книг» and the source line. Each card links to `/top/{slug}`.
- **Link.** The block has the link «Все подборки» to `/top`.
- **Hidden block.** The block MUST render nothing when no collection qualifies or when the collections cannot be loaded.
- **Freshness.** It is not part of the home payload's selection cache: a collection that is published or unpublished shows up or disappears on the next page load.
- No admin setting chooses the collections.

#### Scenario: Three newest collections
- **WHEN** 5 published collections with places exist
- **THEN** «Подборки» shows the 3 most recently first-published ones and the link «Все подборки»

#### Scenario: Empty collection skipped
- **WHEN** the most recently published collection is automatic and has no places yet, and two older published collections have places
- **THEN** «Подборки» shows the two older collections only

#### Scenario: No published collections
- **WHEN** no collection is published
- **THEN** the home page has no «Подборки» block
