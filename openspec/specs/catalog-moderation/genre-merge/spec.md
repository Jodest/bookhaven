# catalog-moderation/genre-merge Specification

## Purpose

Lets an admin merge a duplicate genre into a target genre. Unlike an author merge, the merged genre is kept as an alias, so imports that name it or carry its Wikidata QID later resolve to the target.

## Requirements

### Requirement: Merge endpoint
The system SHALL accept `POST /api/v1/admin/genres/{id}/merge` with `{"canonical_id"}`, where `{id}` is the source genre and `canonical_id` the target. On success it MUST return 200 with `{genre, books_reassigned, collections_switched, merge_id}`: the target genre, the number of books moved to it, the number of curated collections switched to it and the id of the merge journal entry recorded for this merge. A non-UUID id or `canonical_id` MUST return 400, merging a genre into itself MUST return 400, and a missing source or target, or one that is already merged (an alias), MUST return 404; in every error case nothing changes and no journal entry is recorded. The endpoint MUST require the `admin` role and a bearer token.

#### Scenario: Source already merged
- **WHEN** an admin merges a genre that was already merged into another genre
- **THEN** the response is 404 and nothing changes

#### Scenario: Collections switched count
- **WHEN** an admin merges a genre linked to two curated collections
- **THEN** the response has `collections_switched: 2` and both collections are linked to the target

#### Scenario: Merge returns its journal entry
- **WHEN** an admin merges «романы» into «роман»
- **THEN** the response carries a `merge_id`, and the merge history lists that entry with source «романы» and target «роман»

### Requirement: Books move and the source becomes an alias
In one transaction, every book linked to the source SHALL be linked to the target instead; a book already linked to both keeps a single link to the target and is not counted in `books_reassigned`. The source MUST become an alias of the target: it keeps its name and slug, its status becomes `rejected`, and it no longer appears in admin lists, queues, hints or the public site. Aliases that pointed at the source MUST be re-pointed at the target, so an alias always leads directly to a non-alias genre. A merge can be reverted only by undoing its merge journal entry; merges without a journal entry are irreversible.

#### Scenario: Merging a genre that already has an alias
- **WHEN** «романы» was merged into «роман», and then «роман» is merged into «проза»
- **THEN** both «романы» and «роман» are aliases of «проза»

#### Scenario: Imports follow the alias
- **WHEN** after merging «романы» into «роман» a file import row names the genre «романы»
- **THEN** the book is linked to «роман» and no genre is created

### Requirement: Target genre is not edited
The merge SHALL NOT change the target's name, slug or status. A merge into a rejected target is allowed.

#### Scenario: Pending source into published target
- **WHEN** a pending genre is merged into a published genre
- **THEN** the target stays published with its own name

### Requirement: Wikidata QID carry-over
When the source has a Wikidata QID and the target has none, the QID SHALL move to the target. When both have one, the target keeps its own and the source alias keeps its QID, so that QID still resolves to the target.

#### Scenario: Both have QIDs
- **WHEN** a source with QID `Q1` is merged into a target with QID `Q2`
- **THEN** the target keeps `Q2`, and a later import of `Q1` resolves to the target through the alias

### Requirement: Merge dialog
The genre moderation queue SHALL offer «Слить» on every row and «Это он — слить» on each hint (which preselects that hint as the target). The dialog titled «Слить жанр «{source}» в «{target}»» lets the admin search for and change the target, explains that the books move to the target and that the source disappears from the lists while its name and QID keep pointing to the target in future imports, and says that the merge can later be undone in the genre merge history. When the chosen target is rejected it MUST warn «Целевой жанр отклонён — книги окажутся в скрытом жанре.». Once a target is chosen, the dialog MUST show the preview summary: «Будет перенесено книг: {books_reassigned}.» and, for each affected collection, «Подборка «{title}» будет переключена на жанр «{target}».»; while it loads it shows «Считаем последствия…», and the merge button MUST stay disabled until the preview has loaded. A failed preview shows the same error texts as a failed merge. A 400 MUST show «Нельзя слить жанр с самим собой.», a 404 MUST say that one of the genres was not found and may already be merged, and any other error shows «Не удалось выполнить слияние. Попробуйте позже.». After success the queue shows «Жанр «{source}» слит. Перенесено книг: {books_reassigned}.», followed by « Переключено подборок: {collections_switched}.» when that is above zero, and an «Отменить» link that opens the genre merge history with the undo dialog of this merge.

#### Scenario: Rejected target warning
- **WHEN** an admin picks a rejected genre as the target
- **THEN** the dialog warns that the books will end up in a hidden genre

#### Scenario: Collection switch warning
- **WHEN** an admin picks a target for a genre linked to the collection «Лучшие романы»
- **THEN** before confirming, the dialog shows «Подборка «Лучшие романы» будет переключена на жанр «{target}».»

#### Scenario: Undo link after a merge
- **WHEN** an admin merges «романы» into «роман» and clicks «Отменить» in the success message
- **THEN** the genre merge history opens with the undo dialog of that merge

### Requirement: Batch merge endpoint
The system SHALL accept `POST /api/v1/admin/genres/merge-batch` with `{"source_ids": string[], "target_id": string}` and merge every source genre into the target in one transaction: either all sources are merged or nothing changes. On success it MUST return 200 with `{genre, sources_merged, books_reassigned, collections_switched, merge_id}`: the target genre (with a carried QID applied), the number of merged sources, the number of books moved to the target, the number of curated collections switched to it and the id of the single merge journal entry recorded for the whole batch. The whole request MUST be rejected with 400, changing nothing, when the body is malformed, `source_ids` is empty, any id is not a UUID, a source id appears more than once, there are more than 100 source ids, or the target is one of the sources. When any source or the target does not exist or is already merged (an alias), the response MUST be 404 and nothing changes. In every error case no journal entry is recorded. The endpoint MUST require the `admin` role and a bearer token; a moderator MUST get 403.

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
The genre moderation queue SHALL offer «Слить выбранные ({N})» for the selected rows. It opens the dialog «Слить жанры ({N})», which lists the selected genres with a radio button each to choose the genre that stays, plus the option «Другой жанр…» that shows a name search («Поиск жанра по названию») over genres outside the selection. The sources are the selected genres except the chosen target, sent in the queue's order. The dialog MUST explain that the books of the sources move to the target and that the sources disappear from the lists while their names and QIDs keep pointing to the target in future imports, and that the merge can later be undone in the genre merge history; it MUST show the rejected-target warning as the single merge dialog does. Once a target is chosen and there is at least one source, the dialog MUST show the preview summary (as in the merge dialog) and enable «Слить {sources} в «{target}»» only when the preview has loaded. After success it MUST clear the selection and show «Слито жанров: {sources_merged} в «{target}». Перенесено книг: {books_reassigned}.», followed by « Переключено подборок: {collections_switched}.» when that is above zero, and an «Отменить» link that opens the genre merge history with the undo dialog of this merge. A 404 MUST say that one of the genres was not found and may already be merged, a 400 MUST show «Некорректный выбор жанров для слияния.», and any other error shows «Не удалось выполнить слияние. Попробуйте позже.».

#### Scenario: Target picked among the selection
- **WHEN** an admin selects «романы», «роман» and «роман-эпопея» and picks «роман» with the radio button
- **THEN** the dialog previews and merges «романы» and «роман-эпопея» into «роман»

#### Scenario: Target outside the selection
- **WHEN** an admin selects two genres, chooses «Другой жанр…» and picks «проза» from the search
- **THEN** both selected genres are merged into «проза»

#### Scenario: Only the target selected
- **WHEN** an admin selects one genre and picks it with the radio button
- **THEN** there is no source and the merge button stays disabled

### Requirement: Merge journal
Every successful single or batch merge SHALL record, in the same transaction, exactly one merge journal entry: a batch merge is one entry holding all of its sources in request order. The entry MUST record the admin who performed the merge (when known), the time, the target with its name, and for each source: its name, its status and Wikidata QID before the merge, whether its QID moved to the target, the books moved from it to the target, the books whose link to it was dropped because they were already linked to the target at that moment, the aliases re-pointed from it to the target, and the curated collections switched from it to the target. A failed merge MUST record nothing. Merges performed before the journal existed have no entry and MUST NOT be undoable.

#### Scenario: Batch merge is one journal entry
- **WHEN** an admin batch-merges «романы» and «роман-эпопея» into «роман»
- **THEN** the merge history gains one entry with both sources

#### Scenario: Book shared by two sources is recorded per source
- **WHEN** a book is linked to sources A and B (not to the target) and A, B are batch-merged in that order
- **THEN** the entry records the book as moved for A and as an already-linked book for B

#### Scenario: Merge made before the journal
- **WHEN** a genre was merged before the journal existed
- **THEN** that merge is absent from the merge history and cannot be undone

### Requirement: Merge history endpoint
The system SHALL accept `GET /api/v1/admin/genres/merges?page=&limit=` and return 200 with `{items, total, page, limit}`, newest merge first (default `limit` 20, at most 100). Each item MUST be `{id, created_at, performed_by, target: {id, name}, sources: [{id, name}], books_reassigned, collections_switched, undone_at, undone_by}`, where sources are in merge order, names are as recorded at merge time, `performed_by` and `undone_by` are `{id, name}` (the user's name, or e-mail when the name is empty) or `null` when unknown, and `undone_at` is `null` for a merge that was not undone. The endpoint MUST require the `admin` role and a bearer token; a moderator MUST get 403.

#### Scenario: Undone merge in the history
- **WHEN** an admin undoes a merge and then lists the merge history
- **THEN** the entry has `undone_at` set and `undone_by` naming that admin

#### Scenario: Moderator cannot list merges
- **WHEN** a moderator calls `GET /api/v1/admin/genres/merges`
- **THEN** the response is 403

### Requirement: Merge undo endpoint
The system SHALL accept `POST /api/v1/admin/genres/merges/{id}/undo` without a body and undo the whole journal entry — every source of a batch — in one transaction: either everything is undone or nothing changes. On success it MUST return 200 with `{genre, sources_restored, books_returned, books_relinked, collections_switched_back}`: the target genre as it is after the undo, the number of restored sources, the number of moved books returned to their source, the number of books that got their source link back while staying in the target, and the number of collections switched back. The entry MUST then be marked undone with the time and the admin. A non-UUID id MUST return 400, an unknown id 404, and a blocked undo 409 with `{error, code}`; in every error case nothing changes. Automatic collections switched back MUST be recomputed after the undo as after a merge; if that fails, the undo still succeeds. The endpoint MUST require the `admin` role and a bearer token; a moderator MUST get 403.

#### Scenario: Undo a batch merge
- **WHEN** an admin undoes the batch merge of «романы» and «роман-эпопея» into «роман»
- **THEN** the response is 200 with `sources_restored: 2` and both genres are regular genres again

#### Scenario: Unknown merge
- **WHEN** an admin undoes a merge id that does not exist
- **THEN** the response is 404 and nothing changes

#### Scenario: Moderator cannot undo
- **WHEN** a moderator calls `POST /api/v1/admin/genres/merges/{id}/undo`
- **THEN** the response is 403 and nothing changes

### Requirement: Undo restores the recorded state
For every source of the entry, the undo SHALL make the source a regular genre again (not an alias) with its recorded previous status, name and slug, and restore its recorded QID: when the QID had moved to the target, it is taken back from the target only if the target still holds it, otherwise it is restored if no genre holds it. Each recorded moved book that still exists and is still linked to the target MUST be linked to the source again and unlinked from the target — unless a later merge into the same target that is not undone recorded that book, in which case the target link stays; a moved book no longer linked to the target is skipped. Each recorded already-linked book that still exists MUST get its source link back and keep its target link. Each recorded alias that still points at the target MUST point at its source again. Each recorded collection that is still linked to the target MUST be linked to its source again; collections linked elsewhere or deleted are skipped. Books, links and collections changed after the merge are never otherwise touched.

#### Scenario: Source becomes a genre again
- **WHEN** a pending genre «романы» merged into «роман» is undone
- **THEN** «романы» is pending, shows in the genre queue again and its moved books are linked to it and not to «роман»

#### Scenario: Book already in the target keeps both links
- **WHEN** a book linked to both «романы» and «роман» lost its «романы» link in the merge, and the merge is undone
- **THEN** the book is linked to both «романы» and «роман»

#### Scenario: Book removed from the target after the merge
- **WHEN** after the merge an admin unlinks a moved book from «роман», and then the merge is undone
- **THEN** that book is linked to neither genre and the undo succeeds

#### Scenario: Book deleted after the merge
- **WHEN** a moved book is deleted after the merge, and then the merge is undone
- **THEN** the undo succeeds and the other books return

#### Scenario: Carried QID returns to the source
- **WHEN** a source with QID `Q1` was merged into a target without a QID, and the merge is undone
- **THEN** the target has no QID and the source has `Q1` again

#### Scenario: Alias of the source returns
- **WHEN** «романа» was an alias of «романы», was re-pointed to «роман» by the merge, and the merge is undone
- **THEN** «романа» is an alias of «романы» again

#### Scenario: Collection reconfigured after the merge
- **WHEN** a collection switched by the merge was later linked to «проза», and the merge is undone
- **THEN** the collection stays linked to «проза»

#### Scenario: Automatic collection follows the undo
- **WHEN** an automatic collection switched to «роман» by the merge is still linked to it, and the merge is undone
- **THEN** it is linked to «романы» again and is recomputed over the books of «романы»

#### Scenario: Later merge relies on a moved book
- **WHEN** merge 1 moved a book from «романы» into «роман», merge 2 then dropped the book's «новеллы» link because it was already in «роман», and merge 1 is undone
- **THEN** the book is linked to «романы» and still linked to «роман»

### Requirement: Blocked undo
The undo SHALL be refused with 409, changing nothing, when the first of these checks fails, with the `code` given: the entry was already undone (`already_undone`); the target no longer exists (`target_missing`); the target has since been merged into another genre (`target_merged`); a source no longer exists (`source_missing`); a source is no longer an alias of the target (`source_not_alias`); a source's recorded QID is now held by a genre other than the target (`qid_conflict`). A slug conflict cannot occur, because an alias keeps its slug reserved, so it has no code.

#### Scenario: Undo twice
- **WHEN** an admin undoes a merge that was already undone
- **THEN** the response is 409 with `code: already_undone` and nothing changes

#### Scenario: Target merged away
- **WHEN** «романы» was merged into «роман», then «роман» was merged into «проза», and the first merge is undone
- **THEN** the response is 409 with `code: target_merged` and nothing changes

#### Scenario: Target deleted
- **WHEN** the target genre of a merge no longer exists
- **THEN** undoing the merge returns 409 with `code: target_missing`

#### Scenario: Source deleted
- **WHEN** a source genre of a batch merge no longer exists
- **THEN** undoing the merge returns 409 with `code: source_missing` and no other source is restored

#### Scenario: Source points to another genre
- **WHEN** a source of the merge is now an alias of a genre other than the target
- **THEN** undoing the merge returns 409 with `code: source_not_alias`

#### Scenario: QID taken by another genre
- **WHEN** the QID that moved from the source to the target is now held by a third genre
- **THEN** undoing the merge returns 409 with `code: qid_conflict` and nothing changes

### Requirement: Undo preview endpoint
The system SHALL accept `GET /api/v1/admin/genres/merges/{id}/undo-preview` and MUST change nothing. It MUST return 200 with `{merge, books_returned, books_relinked, collections}`: `merge` is the entry as in the merge history, `books_returned` and `books_relinked` are the values an undo would report if nothing changed in between, and `collections` lists the recorded collections still linked to the target as `{id, title, slug, kind, is_published, source_genre: {id, name}}` ordered by title, where `source_genre` is the genre the collection returns to. It MUST return the same 400, 404 and 409 (with `code`) as the undo. The endpoint MUST require the `admin` role and a bearer token.

#### Scenario: Preview counts only books still in the target
- **WHEN** a merge moved 5 books and an admin later unlinked 2 of them from the target
- **THEN** the preview returns `books_returned: 3`

#### Scenario: Preview of a blocked undo
- **WHEN** an admin previews the undo of a merge whose target was merged away
- **THEN** the response is 409 with `code: target_merged`

#### Scenario: Undo preview changes nothing
- **WHEN** an admin requests an undo preview
- **THEN** no genre, book link, collection or journal entry changes

### Requirement: Genre merge history page
The admin app SHALL provide the page «История слияний жанров» at `/moderation/genres/merges`, reachable through the link «История слияний» on the genre moderation queue. It lists merges newest first with pagination and the columns «Когда», «Кто» (the admin's name, or «—» when unknown), «Слияние» (the source names in quotes, joined by commas, then « → » and the target name in quotes), «Книг перенесено» and «Статус» («Активно», or «Отменено {date}» for an undone merge). Every merge that is not undone MUST offer «Отменить слияние». An empty history shows «Слияний пока нет.», and a failed load shows «Не удалось загрузить историю слияний.». Opening the page with `?undo={id}` MUST open the undo dialog of that merge.

#### Scenario: Undone merge has no undo button
- **WHEN** the history shows a merge that was already undone
- **THEN** its status is «Отменено {date}» and it has no «Отменить слияние» button

#### Scenario: History linked from the queue
- **WHEN** an admin clicks «История слияний» on the genre moderation queue
- **THEN** the genre merge history page opens

### Requirement: Undo dialog
The undo dialog SHALL be titled «Отменить слияние в «{target}»» and explain that the source genres become separate genres again with their previous status and that their books, aliases and collections return to them, while changes made after the merge are kept. It MUST show the undo preview: «Считаем последствия…» while loading, then «Вернётся книг в исходные жанры: {books_returned}.», followed by «Книг, которые снова получат исходный жанр, оставаясь в «{target}»: {books_relinked}.» when that is above zero, and «Подборка «{title}» вернётся к жанру «{source}».» for each listed collection. The button «Отменить слияние» MUST stay disabled until the preview has loaded; «Закрыть» closes the dialog. A 409 from the preview or the undo MUST show the text for its code: `already_undone` «Это слияние уже отменено.», `target_missing` «Целевой жанр удалён — отменить слияние нельзя.», `target_merged` «Целевой жанр с тех пор слит с другим жанром — сначала отмените более позднее слияние.», `source_missing` «Один из исходных жанров удалён — отменить слияние нельзя.», `source_not_alias` «Один из исходных жанров больше не связан с целевым — отменить слияние нельзя.», `qid_conflict` «QID одного из исходных жанров теперь занят другим жанром — отменить слияние нельзя.»; a 404 shows «Слияние не найдено. Обновите список.» and any other error «Не удалось отменить слияние. Попробуйте позже.». After success the dialog closes, the history refreshes and shows «Слияние отменено. Восстановлено жанров: {sources_restored}. Вернулось книг: {books_returned}.», followed by « Возвращено подборок: {collections_switched_back}.» when that is above zero.

#### Scenario: Blocked undo in the dialog
- **WHEN** an admin opens the undo dialog of a merge whose target was merged away
- **THEN** the dialog shows «Целевой жанр с тех пор слит с другим жанром — сначала отмените более позднее слияние.» and the undo button stays disabled

#### Scenario: Successful undo
- **WHEN** an admin confirms the undo of a merge that moved 4 books and switched one collection
- **THEN** the history shows «Слияние отменено. Восстановлено жанров: 1. Вернулось книг: 4. Возвращено подборок: 1.» and the entry is «Отменено {date}»
