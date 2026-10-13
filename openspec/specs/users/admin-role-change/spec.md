# users/admin-role-change Specification

## Purpose

Lets an admin give or take away roles while keeping at least one admin and stopping admins from changing their own role.

## Requirements

### Requirement: Changing a role
`PATCH /api/v1/admin/users/{id}/role` `{role}` SHALL require the `admin` role and a bearer header (see `auth/authorization`), set the user's role and return the updated user in the list item shape. The role MUST be one of the user roles, otherwise 400 with code `invalid_role`; an unknown user returns 404 with code `user_not_found`. Setting the current role changes nothing and returns 200. The new role applies from the user's next request; nothing else on the account changes.

#### Scenario: Promoting a reader
- **WHEN** an admin sets a reader's role to `moderator`
- **THEN** the response is 200 and the user can moderate reviews on their next request

#### Scenario: Same role again
- **WHEN** an admin sets a moderator's role to `moderator`
- **THEN** the response is 200 and nothing is written

### Requirement: Own role and last admin
An admin MUST NOT change their own role: 409 with code `cannot_change_own_role`. Demoting the only admin MUST be refused with 409 and code `last_admin`.

#### Scenario: Demoting oneself
- **WHEN** an admin tries to set their own role to `user`
- **THEN** the response is 409 with code `cannot_change_own_role`

### Requirement: Concurrent changes
Role changes SHALL be decided against the current set of admins at the moment of writing, so two concurrent demotions cannot leave the site without an admin. If the acting admin has lost the `admin` role by the time the change is written, the request MUST be refused with 403 and change nothing. Every effective change is logged with the acting admin, the user and the old and new roles.

#### Scenario: Two admins demote each other
- **WHEN** the only two admins try to demote each other at the same time
- **THEN** one change succeeds and the other is refused, so one admin remains

### Requirement: Role control on the users page
Each row of «Пользователи» SHALL have a role selector. Picking a different role asks «Изменить роль пользователя «<name or email>»: <old> → <new>?», adding «Пользователь получит полный доступ к админ-панели.» when the new role is admin. Cancelling keeps the old role. On success the page shows «Роль пользователя «<name or email>» изменена: <old> → <new>.»; on failure the selector returns to the old role and the page shows «Нельзя снять роль с последнего администратора.», «Нельзя изменить собственную роль.», «Пользователь не найден — обновите список.», «Недопустимая роль.» or «Не удалось изменить роль. Попробуйте ещё раз.». The admin's own row has its selector disabled and is marked «Это вы».

#### Scenario: Cancelling the confirmation
- **WHEN** an admin picks «Модератор» for a reader and cancels the confirmation
- **THEN** the selector shows «Читатель» again and no request is sent
