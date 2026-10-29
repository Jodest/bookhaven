## MODIFIED Requirements

### Requirement: Lookup errors
The lookup MUST return 400 when neither `isbn` nor `q` is given, 404 when the source has no match, and 502 when Open Library is rate-limited or unavailable. A title search answer that reports more than 10 000 matching books MUST be treated as no match (404), because it means Open Library ignored the query. The lookup does not otherwise check that the returned title or author matches the request: the record is a prefill the admin reviews.

#### Scenario: Source unavailable
- **WHEN** Open Library times out after retries
- **THEN** the response is 502 with a message to retry or enter the data manually

#### Scenario: Query ignored by Open Library
- **WHEN** an admin requests `?q=История Петра I&author=Александр Сергеевич Пушкин` and Open Library answers with 907 849 matching books, the first being "I, Robot"
- **THEN** the response is 404 and the form shows the lookup-miss message
