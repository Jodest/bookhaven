## MODIFIED Requirements

### Requirement: Personal blocks on the page
For signed-in users the home page SHALL show in the main column (see `home/home-page`): «Продолжить чтение» with «Все (<total>)» linking to the shelf at the top; «Из вашего «Хочу прочитать»» with «Вся полка (<total>)» under «Книга дня»; and further down «Вам может понравиться». The reading-challenge card (see `reading-challenges/progress`) comes first in the sidebar. Guests and visitors whose session is still being checked see none of them; loading blocks show placeholders, and empty or failed blocks are hidden.

#### Scenario: Guest sees no personal blocks
- **WHEN** a guest opens `/`
- **THEN** no personal block is shown and `GET /api/v1/home/me` is not called
