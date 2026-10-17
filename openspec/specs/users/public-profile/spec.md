# users/public-profile Specification

## Purpose

Shows anyone a reader's public face: name, avatar, profile type, how long they have been on the site, and their public shelves, without exposing private account data.

## Requirements

### Requirement: Public profile endpoint
`GET /api/v1/users/{id}` SHALL be available to anyone, signed in or not, and return `{user, shelves}`. `user` is `{id, name, avatar_url, profile_type, joined_at}`, where `joined_at` is the account creation time. `shelves` follows `collections/public-collections`; each item is `{id, name, type, description, books_count, created_at}`, and the array is empty when there is nothing to show. The response has no `lists` field. A malformed id MUST return 400 and an unknown user 404.

#### Scenario: Anonymous request
- **WHEN** a client without a token requests a reader's profile
- **THEN** the profile is returned

#### Scenario: Unknown user
- **WHEN** anyone requests the profile of an id that has no account
- **THEN** the response is 404

### Requirement: No private data
The public profile MUST NOT include the email, role, email confirmation state or any other account field beyond those listed above. It is returned for every existing account, including accounts whose email is not confirmed yet.

#### Scenario: Email stays private
- **WHEN** anyone requests a reader's public profile
- **THEN** the response has no email and no role

### Requirement: Public profile page
The web app SHALL provide `/profile/{id}` for everyone, signed in or not (see `auth/authorization`). It shows the name (or «Без имени»), the avatar (or the first letter of the name), the profile type («Читатель», «Библиотекарь», «Книготорговец», «Другое») and «На сайте с <date>». Below that comes «Полки» («Публичных полок пока нет»): each shelf has its book count, its description when set, and a link to its public shelf page. Default shelves are shown under their Russian names, custom shelves under their own names. There is no «Публичные списки» block. Errors show «Некорректный идентификатор пользователя», «Профиль не найден или недоступен» or «Ошибка загрузки профиля». No page of the site links to a public profile yet; it is reached by its address.

#### Scenario: Visitor opens a profile
- **WHEN** a visitor opens `/profile/{id}` of a reader with one public custom shelf holding a published book
- **THEN** the page shows the reader's name, «На сайте с …» and the shelf with a link to `/shelves/public/{shelf id}`

#### Scenario: Profile does not exist
- **WHEN** a visitor opens `/profile/{id}` for an unknown id
- **THEN** «Профиль не найден или недоступен» is shown
