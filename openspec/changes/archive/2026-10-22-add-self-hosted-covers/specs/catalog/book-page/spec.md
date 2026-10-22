## MODIFIED Requirements

### Requirement: Search and preview metadata
The book page SHALL provide its own document metadata.
- **Title.** The page title is «{title} — {authors, comma-separated} — BookHaven». When the book has no authors it is «{title} — BookHaven».
- **Description.** The meta description is the book's own description cut to about 160 characters at a word boundary, with «…» added when cut. Without an own description it is «{title}: оценки, рецензии и экранизации на BookHaven.».
- **Open Graph.** The Open Graph data repeats the title and the description. The book cover is the Open Graph image when the book has one. The image MUST be an absolute URL: a self-hosted cover (`/covers/…`) is prefixed with the public site's origin.
- **Structured data.** The page embeds schema.org `Book` structured data with:
  - the name and the authors (as `Person`);
  - the ISBN, the page count, the publication year and the language, when known;
  - the cover image, when the book has one, as the same absolute URL as the Open Graph image;
  - an `aggregateRating` (`ratingValue`, `ratingCount`, best 5, worst 1) only when the book has at least one rating.
- **No AI text.** An AI-generated description MUST NOT appear in the page metadata or the structured data.
- **Unknown book.** An unknown or unpublished book keeps the not-found page.

#### Scenario: Metadata of a rated book
- **WHEN** a crawler fetches a published book with a cover, two authors and 12 ratings
- **THEN** the title names the book and both authors, the cover is the Open Graph image, and the structured data has `aggregateRating.ratingCount` 12

#### Scenario: No ratings, no aggregate
- **WHEN** a crawler fetches a published book without ratings
- **THEN** the structured data has no `aggregateRating`

#### Scenario: Only an AI description
- **WHEN** a published book has only an approved AI description
- **THEN** the meta description is «{title}: оценки, рецензии и экранизации на BookHaven.» and the structured data has no description

#### Scenario: Self-hosted cover in metadata
- **WHEN** a crawler fetches a published book whose cover is served from `/covers/ab/abc….jpg` on the site `https://bookhaven.example`
- **THEN** the Open Graph image and the structured-data image are both `https://bookhaven.example/covers/ab/abc….jpg`
