## MODIFIED Requirements

### Requirement: Queue filters
Each queue SHALL accept `search` and these filters, which combine with the status and with each other:
- authors: `qid=with|without` (has a Wikidata QID or not) and `books=none` (authors with no linked books); `search` is a full-text search on name and biography;
- genres: `qid=with|without`; `search` matches the name as a substring;
- books: `source=manual|harvested` (no external source, or created by the harvest) and `ai=publish|reject|needs_human|none` (the AI verdict, stale or not, or `none` for books without any verdict, see `catalog-moderation/ai-verdicts`); `search` is a full-text search on title, ISBN and description.

#### Scenario: Authors without books
- **WHEN** an admin requests `?books=none`
- **THEN** only pending authors with no linked books are listed

#### Scenario: Harvested pending books
- **WHEN** an admin requests `?status=pending&source=harvested`
- **THEN** only pending books created by the harvest are listed

#### Scenario: Books the AI would publish
- **WHEN** an admin requests `?status=pending&ai=publish`
- **THEN** only pending books whose current AI verdict is `publish` are listed

#### Scenario: Books never checked by the AI
- **WHEN** an admin requests `?ai=none`
- **THEN** only books without any AI verdict are listed

#### Scenario: Unknown AI filter
- **WHEN** an admin requests `GET /api/v1/admin/books/moderation?ai=maybe`
- **THEN** the response is 400

### Requirement: Book context
Each book queue item SHALL carry the book with its authors and genres (each with its own status), its `created_at`, its external source and `external_id`, `similar` hints (see `catalog-moderation/duplicate-hints`) and `ai_verdict`: `null` when the book has no verdict, otherwise `{verdict, reason_codes, reason, confidence, model, prompt_version, created_at, stale}` (see `catalog-moderation/ai-verdicts`).

#### Scenario: Harvested book
- **WHEN** a harvested pending book is listed
- **THEN** the item shows its source and external ID

#### Scenario: Book without a verdict
- **WHEN** a pending book that was never checked by the AI is listed
- **THEN** the item has `ai_verdict: null`

#### Scenario: Book with a stale verdict
- **WHEN** a pending book was edited after its AI verdict was computed
- **THEN** the item's `ai_verdict` has `stale: true`
