## MODIFIED Requirements

### Requirement: Adaptations section on the book edit page
The admin book edit page SHALL show a section «Экранизации» listing every linked adaptation with its title, year, type and directors. Hidden ones carry the mark «Скрыта», and manual links carry the mark «Вручную». Each row has:
- «Скрыть», or «Показать» for a hidden one, which applies at once and refreshes the list;
- «Изменить», which opens the work's edit page and returns to the book afterwards;
- for a manual link only, «Удалить», which asks «Удалить связь с «<title>»? Сам фильм останется в каталоге.» and then deletes the link and refreshes the list.

The section always offers «Добавить экранизацию» (see `screen-works/admin-editing`), and offers «Найти экранизации в Wikidata» when the book has a Wikidata QID. It shows:
- «Экранизаций пока нет.» with no links;
- «Не удалось загрузить экранизации.» when the list fails to load;
- «Не удалось изменить видимость экранизации.» when hiding or showing fails;
- «Не удалось удалить связь.» when deleting fails.

#### Scenario: Hiding from the edit page
- **WHEN** an admin clicks «Скрыть» on an adaptation in the section
- **THEN** the row shows «Скрыта» and the button becomes «Показать»

#### Scenario: Wikidata row has no delete
- **WHEN** the section lists one adaptation from the sync and one added by hand
- **THEN** only the manual one has «Вручную» and «Удалить», and both have «Скрыть» and «Изменить»

#### Scenario: Book without a QID
- **WHEN** an admin opens the edit page of a book without a Wikidata QID
- **THEN** the section has «Добавить экранизацию» and no «Найти экранизации в Wikidata»
