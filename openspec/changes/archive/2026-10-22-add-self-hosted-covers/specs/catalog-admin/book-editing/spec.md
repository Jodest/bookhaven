## ADDED Requirements

### Requirement: Cover link and copy on the admin book record
The admin book record SHALL return the stored source link in `cover_url`, never a `/covers/…` path. This record is what the admin book list and `GET`, `POST` and `PATCH /api/v1/admin/books[/{id}]` return, so saving the edit form never replaces the source with our copy. The record also carries `cover_copy`: `null` when the book has no cover link, otherwise an object with:
- `status`: `pending`, `stored`, `failed` or `absent`. It is `pending` when no copy of the current link exists yet;
- `error`: one of `host_not_allowed`, `blocked_address`, `too_many_redirects`, `too_large`, `not_image`, `http_error`, `timeout`, `network`, or `null`;
- `stored_at`: the time the copy was stored, or `null`.

These endpoints keep their current role and bearer requirements.

#### Scenario: Editing a copied book
- **WHEN** an admin opens and saves, unchanged, a book whose Open Library cover has a stored copy
- **THEN** the form shows and sends the Open Library link, and the book keeps its stored copy

#### Scenario: Copy status in the record
- **WHEN** an admin fetches a book whose cover host is not on the allowlist
- **THEN** `cover_copy` is `{"status": "failed", "error": "host_not_allowed", "stored_at": null}`

### Requirement: Cover copy hint on the edit page
On «Редактирование книги», a hint SHALL appear under «URL обложки» when the loaded book has a cover link:
- `stored`: «Копия обложки сохранена на нашем сервере.»
- `pending`: «Обложка копируется на наш сервер. Пока показывается исходная ссылка.»
- `failed`: «Не удалось скопировать обложку: <reason>. На сайте показывается исходная ссылка.» The reason is:
  - «сайт не входит в список разрешённых» for `host_not_allowed` and `blocked_address`;
  - «файл больше 5 МБ» for `too_large`;
  - «это не изображение JPEG, PNG или WebP» for `not_image`;
  - «источник недоступен» for any other error.
- `absent`: «По этой ссылке нет обложки. На сайте показывается «Нет обложки».»

The hint describes the loaded book. It MUST NOT change while the admin edits the field, and the manual create form shows no hint.

#### Scenario: Failed copy hint
- **WHEN** an admin opens a book whose cover copy failed with `too_large`
- **THEN** the hint reads «Не удалось скопировать обложку: файл больше 5 МБ. На сайте показывается исходная ссылка.»
