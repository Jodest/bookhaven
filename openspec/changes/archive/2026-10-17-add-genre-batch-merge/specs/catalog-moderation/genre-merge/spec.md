## ADDED Requirements

### Requirement: Batch merge endpoint
The system SHALL accept `POST /api/v1/admin/genres/merge-batch` with `{"source_ids": string[], "target_id": string}` and merge every source genre into the target in one transaction: either all sources are merged or nothing changes. On success it MUST return 200 with `{genre, sources_merged, books_reassigned, collections_switched}`: the target genre (with a carried QID applied), the number of merged sources, the number of books moved to the target and the number of curated collections switched to it. The whole request MUST be rejected with 400, changing nothing, when the body is malformed, `source_ids` is empty, any id is not a UUID, a source id appears more than once, there are more than 100 source ids, or the target is one of the sources. When any source or the target does not exist or is already merged (an alias), the response MUST be 404 and nothing changes. The endpoint MUST require the `admin` role and a bearer token; a moderator MUST get 403.

#### Scenario: Three genres into one
- **WHEN** an admin merges «романы», «Роман (жанр)» and «роман-эпопея» into «роман»
- **THEN** the response is 200 with `sources_merged: 3` and all three are aliases of «роман»

#### Scenario: One source already merged
- **WHEN** an admin batch-merges three genres and one of them was merged into another genre a moment before
- **THEN** the response is 404 and none of the three genres changes

#### Scenario: Target among the sources
- **WHEN** an admin sends a target id that is also listed in `source_ids`
- **THEN** the response is 400 and nothing changes

#### Scenario: Duplicate source id
- **WHEN** an admin lists the same source id twice
- **THEN** the response is 400 and nothing changes

#### Scenario: Too many sources
- **WHEN** an admin sends 101 distinct source ids
- **THEN** the response is 400 and nothing changes

#### Scenario: Moderator is refused
- **WHEN** a moderator calls `POST /api/v1/admin/genres/merge-batch`
- **THEN** the response is 403 and nothing changes

### Requirement: Batch merge follows the single merge
Each source in a batch SHALL be merged into the target exactly as a single merge would, one after another in request order: its books move to the target without duplicate links, it becomes a rejected alias of the target keeping its name and slug, and aliases that pointed at it are re-pointed at the target. The target's name, slug and status MUST NOT change. `books_reassigned` MUST equal the number of distinct books that were linked to at least one source and not to the target before the merge. When the target has no Wikidata QID, the QID of the first source in request order that has one SHALL move to the target; every other source keeps its QID on its alias, so those QIDs still resolve to the target.

#### Scenario: Book shared by two sources
- **WHEN** a book is linked to two sources and not to the target, and the sources are batch-merged
- **THEN** the book ends up with one link to the target and counts once in `books_reassigned`

#### Scenario: Several sources with QIDs
- **WHEN** sources with QIDs `Q1` and `Q2` (in that request order) are merged into a target without a QID
- **THEN** the target gets `Q1`, and a later import of `Q2` resolves to the target through the alias

### Requirement: Merge preview endpoint
The system SHALL accept `POST /api/v1/admin/genres/merge-preview` with the same body and the same 400 and 404 validation as the batch merge, and MUST change nothing. It MUST return 200 with `{books_reassigned, collections}`, where `books_reassigned` is the number of distinct books linked to at least one source and not to the target (the value a merge would report if nothing changed in between), and `collections` lists every curated collection linked to one of the sources as `{id, title, slug, kind, is_published, source_genre: {id, name}}`, ordered by title. The endpoint MUST require the `admin` role and a bearer token. A single merge is previewed by sending one source.

#### Scenario: Preview counts only books not yet in the target
- **WHEN** a source has 5 books, 2 of which are already linked to the target
- **THEN** the preview returns `books_reassigned: 3`

#### Scenario: Preview lists a linked collection
- **WHEN** the collection «Лучшие романы» is linked to a source genre «романы»
- **THEN** the preview lists it with `source_genre` «романы»

#### Scenario: Preview changes nothing
- **WHEN** an admin requests a preview
- **THEN** no genre, book link or collection changes

### Requirement: Curated collections follow a merge
In the same transaction as a single or batch merge, every curated collection linked to a source genre SHALL be linked to the target instead; the collection is never deleted and a linked collection never blocks a merge. Each switched automatic collection MUST then be recomputed at once without a movement baseline, as when an admin changes its genre; if that recomputation fails, the merge still succeeds and the collection keeps its previous snapshot until the next scheduled recomputation. Collections that were still linked to a genre merged before this behavior existed SHALL be switched to that genre's target once.

#### Scenario: Automatic collection keeps ranking
- **WHEN** «романы» is merged into «роман» and an automatic collection is linked to «романы»
- **THEN** the collection is linked to «роман» and is recomputed over the books of «роман», which now include those of «романы»

#### Scenario: Merge into a rejected target
- **WHEN** a genre linked to a published collection is merged into a rejected genre
- **THEN** the collection is switched to the rejected genre and its public page shows no genre

### Requirement: Batch merge dialog
The genre moderation queue SHALL offer «Слить выбранные ({N})» for the selected rows. It opens the dialog «Слить жанры ({N})», which lists the selected genres with a radio button each to choose the genre that stays, plus the option «Другой жанр…» that shows a name search («Поиск жанра по названию») over genres outside the selection. The sources are the selected genres except the chosen target, sent in the queue's order. The dialog MUST explain that the books of the sources move to the target and that the sources disappear from the lists while their names and QIDs keep pointing to the target in future imports, and that the action is irreversible; it MUST show the rejected-target warning as the single merge dialog does. Once a target is chosen and there is at least one source, the dialog MUST show the preview summary (as in the merge dialog) and enable «Слить {sources} в «{target}»» only when the preview has loaded. After success it MUST clear the selection and show «Слито жанров: {sources_merged} в «{target}». Перенесено книг: {books_reassigned}.», followed by « Переключено подборок: {collections_switched}.» when that is above zero. A 404 MUST say that one of the genres was not found and may already be merged, a 400 MUST show «Некорректный выбор жанров для слияния.», and any other error shows «Не удалось выполнить слияние. Попробуйте позже.».

#### Scenario: Target picked among the selection
- **WHEN** an admin selects «романы», «роман» and «роман-эпопея» and picks «роман» with the radio button
- **THEN** the dialog previews and merges «романы» and «роман-эпопея» into «роман»

#### Scenario: Target outside the selection
- **WHEN** an admin selects two genres, chooses «Другой жанр…» and picks «проза» from the search
- **THEN** both selected genres are merged into «проза»

#### Scenario: Only the target selected
- **WHEN** an admin selects one genre and picks it with the radio button
- **THEN** there is no source and the merge button stays disabled

## MODIFIED Requirements

### Requirement: Merge endpoint
The system SHALL accept `POST /api/v1/admin/genres/{id}/merge` with `{"canonical_id"}`, where `{id}` is the source genre and `canonical_id` the target. On success it MUST return 200 with `{genre, books_reassigned, collections_switched}`: the target genre, the number of books moved to it and the number of curated collections switched to it. A non-UUID id or `canonical_id` MUST return 400, merging a genre into itself MUST return 400, and a missing source or target, or one that is already merged (an alias), MUST return 404; in every error case nothing changes. The endpoint MUST require the `admin` role and a bearer token.

#### Scenario: Source already merged
- **WHEN** an admin merges a genre that was already merged into another genre
- **THEN** the response is 404 and nothing changes

#### Scenario: Collections switched count
- **WHEN** an admin merges a genre linked to two curated collections
- **THEN** the response has `collections_switched: 2` and both collections are linked to the target

### Requirement: Merge dialog
The genre moderation queue SHALL offer «Слить» on every row and «Это он — слить» on each hint (which preselects that hint as the target). The dialog titled «Слить жанр «{source}» в «{target}»» lets the admin search for and change the target, explains that the books move to the target and that the source disappears from the lists while its name and QID keep pointing to the target in future imports, and says the action is irreversible. When the chosen target is rejected it MUST warn «Целевой жанр отклонён — книги окажутся в скрытом жанре.». Once a target is chosen, the dialog MUST show the preview summary: «Будет перенесено книг: {books_reassigned}.» and, for each affected collection, «Подборка «{title}» будет переключена на жанр «{target}».»; while it loads it shows «Считаем последствия…», and the merge button MUST stay disabled until the preview has loaded. A failed preview shows the same error texts as a failed merge. A 400 MUST show «Нельзя слить жанр с самим собой.», a 404 MUST say that one of the genres was not found and may already be merged, and any other error shows «Не удалось выполнить слияние. Попробуйте позже.». After success the queue shows «Жанр «{source}» слит. Перенесено книг: {books_reassigned}.», followed by « Переключено подборок: {collections_switched}.» when that is above zero.

#### Scenario: Rejected target warning
- **WHEN** an admin picks a rejected genre as the target
- **THEN** the dialog warns that the books will end up in a hidden genre

#### Scenario: Collection switch warning
- **WHEN** an admin picks a target for a genre linked to the collection «Лучшие романы»
- **THEN** before confirming, the dialog shows «Подборка «Лучшие романы» будет переключена на жанр «{target}».»
