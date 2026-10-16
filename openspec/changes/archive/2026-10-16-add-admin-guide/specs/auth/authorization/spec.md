## MODIFIED Requirements

### Requirement: Admin app access
Every admin page except `/login` SHALL redirect a visitor with neither an `access_token` nor a `has_session` cookie to `/login?redirect=<path>`. Only `admin` and `moderator` users may use the admin app: any other signed-in user MUST see «Доступ запрещен» («У вас недостаточно прав для доступа к панели администратора.»). Sections are shown according to role (a moderator sees review moderation and, in «Справка», only the chapter «Модерация отзывов»), and when the server answers 403 to a request on the current page, the page shows that the server rejected the request for lack of rights.

#### Scenario: Reader opens the admin app
- **WHEN** a user with the role `user` signs in to the admin app
- **THEN** the admin app shows «Доступ запрещен»

#### Scenario: Moderator sidebar
- **WHEN** a moderator signs in to the admin app
- **THEN** the sidebar shows only «Модерация» and «Справка»
