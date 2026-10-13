# users/own-profile Specification

## Purpose

Lets a signed-in user change how they appear on the site: their name, an avatar link and a profile type.

## Requirements

### Requirement: Updating my profile
`PATCH /api/v1/users/me` SHALL require authentication and update only the caller's own account. It accepts any of `{name, avatar_url, profile_type}`; a field that is left out keeps its value. It returns the updated user in the same shape as `GET /api/v1/auth/me` (see `auth/session`).

#### Scenario: Partial update
- **WHEN** a user sends only a new `name`
- **THEN** the name changes and the avatar link and profile type stay as they were

#### Scenario: Not signed in
- **WHEN** a request without a valid access token calls `PATCH /api/v1/users/me`
- **THEN** the response is 401 and no profile changes

### Requirement: Field rules
The name SHALL be trimmed and MUST be 2–255 characters. The avatar link MUST be an absolute `http` or `https` URL with a host; an empty string or `null` removes the avatar. The profile type MUST be one of `reader`, `librarian`, `bookseller`, `other`. Any violation MUST return 400 and change nothing. A name cannot be removed, only replaced.

#### Scenario: Removing the avatar
- **WHEN** a user sends `avatar_url: null`
- **THEN** the profile has no avatar link

#### Scenario: Invalid avatar link
- **WHEN** a user sends `avatar_url: "ftp://example.com/a.png"`
- **THEN** the response is 400 and the profile is unchanged

### Requirement: Only profile fields change
A profile update MUST write only the name, avatar link, profile type and update time. Email, password, email confirmation and role MUST NOT change through it, even if the request contains them, and a profile save MUST NOT undo a role change an admin made at the same time.

#### Scenario: Role in the request
- **WHEN** a user sends `{"name": "Анна", "role": "admin"}`
- **THEN** the name changes and the role stays as it was

### Requirement: Profile page
The web app SHALL provide `/profile` for signed-in users (see `auth/authorization` for the guard) with the form «Профиль»: «Имя», «Ссылка на аватар» with a preview (the first letter of the name or email when there is no link), «Тип профиля» («Читатель», «Библиотекарь», «Книготорговец», «Другое»), the read-only email, «Сохранить» and «Выйти». The form checks the fields before sending («Имя должно содержать от 2 до 255 символов», «Ссылка на аватар должна начинаться с http:// или https://», «Введите корректную ссылку на аватар»), shows «Профиль сохранён» on success and «Не удалось сохранить профиль. Попробуйте позже.» on an unexpected failure, and updates the name shown in the header. Clearing the avatar field removes the avatar.

#### Scenario: Saving the profile
- **WHEN** a user changes their name to «Анна» and clicks «Сохранить»
- **THEN** «Профиль сохранён» is shown and the header shows «Анна»

#### Scenario: Name too short
- **WHEN** a user enters a one-letter name and clicks «Сохранить»
- **THEN** «Имя должно содержать от 2 до 255 символов» is shown and nothing is sent
