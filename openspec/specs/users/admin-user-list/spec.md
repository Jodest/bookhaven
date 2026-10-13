# users/admin-user-list Specification

## Purpose

Lets an admin find any account by email, name or role and see its basic state.

## Requirements

### Requirement: Listing users
`GET /api/v1/admin/users` SHALL require the `admin` role and return `{items, total, page, limit}`, newest accounts first. Each item is `{id, email, name, avatar_url, role, profile_type, email_verified_at, created_at}`. `page` defaults to 1 and `limit` to 20, at most 100.

#### Scenario: Moderator asks for users
- **WHEN** a moderator calls `GET /api/v1/admin/users`
- **THEN** the response is 403

### Requirement: Search and role filter
`search` SHALL match the email or the name as a case-insensitive substring. `role` MUST be one of the user roles (see `auth/authorization`) and limits the list to that role; any other value returns 400 with code `invalid_role`. Both can be combined, and `total` counts the filtered accounts.

#### Scenario: Searching by part of an email
- **WHEN** an admin searches for «IVAN» and an account `ivanov@example.com` exists
- **THEN** that account is listed

#### Scenario: Unknown role filter
- **WHEN** an admin calls the list with `role=owner`
- **THEN** the response is 400 with code `invalid_role`

### Requirement: Users page
The admin app SHALL show «Пользователи» to admins only (see `auth/authorization`). The page lists accounts 20 per page with the columns «Имя» («—» when empty), «Email», «Роль», «Email подтверждён» («Да» / «Нет»), «Создан» and «Изменить роль». It has a search box «Поиск по email или имени» and a role filter («Все роли», «Читатель», «Автор контента», «Модератор», «Администратор»). Search, role and page are kept in the address. It shows «Пользователи не найдены.» for an empty result and «Не удалось загрузить пользователей.» on failure.

#### Scenario: Filtering by role
- **WHEN** an admin picks «Модератор» in the role filter
- **THEN** only moderators are listed, starting from the first page, and the address carries `role=moderator`
