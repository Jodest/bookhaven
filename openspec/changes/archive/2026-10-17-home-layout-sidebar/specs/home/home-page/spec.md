## MODIFIED Requirements

### Requirement: Home page layout
The web app SHALL render `/` on the server without a hero: there is no page heading, tagline, call-to-action buttons or genre links, since the header and the catalog cover them. On wide screens the page has two columns. The main column holds, in order: «Продолжить чтение» for signed-in users (see `home/personal-blocks`), «Книга дня», «Из вашего «Хочу прочитать»» for signed-in users, «Вам может понравиться», «Новое в каталоге», «Высоко оценённые», «Подборки» (see `home/catalog-rows`) and «Свежие рецензии». A narrow sidebar next to it holds, for signed-in users, the reading-challenge card, then «Что почитать?» and the literary calendar. On narrow screens the page is one column: «Продолжить чтение», «Книга дня» and «Из вашего «Хочу прочитать»», then the sidebar blocks, then the remaining rows. Book rows show one row of books that fits the column, without horizontal scrolling. An empty block renders nothing. «Подборки» does not come from the home payload. It SHALL be loaded on its own, like the genre list of «Что почитать?». If the home payload cannot be loaded, the page still shows the personal blocks, «Что почитать?» and «Подборки».

#### Scenario: Guest on an empty catalog
- **WHEN** a guest opens `/` while no book is published
- **THEN** the page shows «Что почитать?» and no book block, heading, buttons or genre links

#### Scenario: Home payload fails
- **WHEN** a guest opens `/` while the home payload request fails and one collection with places is published
- **THEN** the page shows «Что почитать?» and the «Подборки» block, and no other book block
