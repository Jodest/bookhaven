## MODIFIED Requirements

### Requirement: Home page layout
The web app SHALL render `/` on the server with the heading «BookHaven», the tagline «Социальный каталог книг с оценками, ревью и подборками», the buttons «Найти книгу» (`/books`) and «Мои полки» (`/shelves`) and up to 12 genre links (`/books?genre_id=…`). Then come the personal blocks for signed-in users (see `home/personal-blocks`), «Книга дня» next to the literary calendar, «Вам может понравиться», «Что почитать?», «Новое в каталоге», «Высоко оценённые», «Подборки» (see `home/catalog-rows`) and «Свежие рецензии». An empty block renders nothing. «Подборки» does not come from the home payload. It SHALL be loaded on its own, like the genre links. If the home payload cannot be loaded, the page still shows the heading, the genre links, the personal blocks and «Подборки».

#### Scenario: Guest on an empty catalog
- **WHEN** a guest opens `/` while no book is published
- **THEN** the page shows the heading and the buttons, and no book block

#### Scenario: Home payload fails
- **WHEN** a guest opens `/` while the home payload request fails and one collection with places is published
- **THEN** the page shows the heading, the genre links and the «Подборки» block, and no other book block
