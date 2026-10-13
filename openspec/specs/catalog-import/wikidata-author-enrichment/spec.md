# catalog-import/wikidata-author-enrichment Specification

## Purpose

Fills missing author dates and photos from Wikidata and Wikimedia Commons on an admin's request, without ever overwriting what is already there.

## Requirements

### Requirement: Enrichment endpoint
`POST /api/v1/admin/authors/enrich-wikidata` `{ids}` SHALL require the `admin` role and a bearer header and enrich 1 to 50 distinct authors of any status, returning 200 with `{processed, enriched, items}`, one item per author: `{author_id, name, outcome, filled, notes}`. Empty `ids`, more than 50 ids or a malformed id return 400. Without a configured Wikidata source the response is 503; when every Wikidata request failed it is 502.

#### Scenario: Too many authors
- **WHEN** an admin sends 51 author ids
- **THEN** the response is 400 and no author is changed

### Requirement: Outcomes
Each author SHALL get one outcome: `enriched` (something was filled), `unchanged` (nothing to fill), `skipped` (no Wikidata QID: note `no_qid`; or the entity does not exist: note `entity_not_found`) or `error` (unknown author, or Wikidata unavailable for its batch: note `wikidata_unavailable`). `filled` lists `birth_date`, `death_date` and `photo` as filled. Notes also report unusable dates (`birth_date_unusable`, `death_date_unusable`) and why a photo was not taken (`photo_unavailable`, `photo_license_not_free`, `photo_attribution_missing`, `commons_unavailable`).

#### Scenario: Author without a QID
- **WHEN** an admin enriches an author that has no Wikidata QID
- **THEN** its outcome is `skipped` with the note `no_qid`

### Requirement: Fill empty fields only
Enrichment SHALL fill only empty fields and MUST NOT overwrite a date or photo already set, including one set by someone else while the enrichment ran. A death date earlier than the birth date is not used. A photo is taken only from a Commons file with a free license and a known attribution, which is stored with the photo (see `catalog-admin/author-editing`). The same rules apply to authors enriched during a direct Wikidata import and after a harvest (see `catalog-import/wikidata-author-import` and `catalog-import/wikidata-book-harvest`).

#### Scenario: Birth date already set
- **WHEN** an author has a birth date and Wikidata has a different one
- **THEN** the birth date stays and is not listed in `filled`

### Requirement: Enrichment actions in the admin
The author list SHALL offer «Обогатить из Wikidata» on rows with a QID, and the author moderation queue offers it for one row and for the selected rows («Обогатить из Wikidata (<n>)», counting only rows with a QID). A single result reads «Автор «<name>»: добавлены <fields>.», «…: нечего добавить — поля уже заполнены или в Wikidata нет подходящих данных.», «…: пропущен — <reason>.» or «…: не удалось обогатить — <reason>.». A bulk result reads «Обогащено <n> из <m>.», adds «Без QID пропущено: <k>.» and up to three per-author details. Failures show «Wikidata недоступна, попробуйте позже.», «Обогащение не настроено.» or «Не удалось обогатить авторов.».

#### Scenario: Bulk enrichment with authors lacking a QID
- **WHEN** an admin selects five authors in the moderation queue, two of them without a QID, and clicks «Обогатить из Wikidata (3)»
- **THEN** three authors are sent and the banner says «Без QID пропущено: 2.»
