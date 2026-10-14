## MODIFIED Requirements

### Requirement: One sub-page per import method
Each entity tab SHALL have one page per available method, in the fixed order Вручную → Wikidata → Из файла → Подбор ИИ: `/import/books/{manual,wikidata,file}`, `/import/authors/{manual,wikidata,ai}` and `/import/genres/{manual,wikidata}`. Only authors support «Подбор ИИ». A method an entity does not support MUST have no tab, route or placeholder. Each sub-page renders only its own method with its own heading. The «Подбор ИИ» sub-tab MUST be shown even when AI curation is switched off; its page then explains that the feature is off.

#### Scenario: Authors have no file import
- **WHEN** an admin opens the Авторы tab
- **THEN** only the «Вручную», «Wikidata» and «Подбор ИИ» sub-tabs are shown, in that order

#### Scenario: Books and genres have no AI curation
- **WHEN** an admin opens the Книги or Жанры tab
- **THEN** no «Подбор ИИ» sub-tab is shown and `/import/books/ai` does not exist
