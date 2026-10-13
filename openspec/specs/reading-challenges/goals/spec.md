# reading-challenges/goals Specification

## Purpose

Lets a reader set how many books they want to read in a year and look back at earlier years.

## Requirements

### Requirement: Setting a goal
`PUT /api/v1/reading-challenges/{year}` `{goal}` SHALL require authentication and create or replace the caller's goal for that year, returning the year's progress (see `reading-challenges/progress`). The goal MUST be a whole number from 1 to 10,000 and the year from 2000 to next year (UTC), otherwise 400 («invalid goal» / «invalid year»). A reader has at most one goal per year; a goal cannot be deleted, only changed.

#### Scenario: Changing the goal
- **WHEN** a reader with a goal of 20 for this year sets 30
- **THEN** the year has one goal, 30

#### Scenario: Year too far ahead
- **WHEN** a reader sets a goal for the year after next
- **THEN** the response is 400 and nothing is saved

### Requirement: Reading a year and the history
`GET /api/v1/reading-challenges/{year}` SHALL return the caller's progress for a year in the allowed range, with `goal: null` when no goal is set; `GET /api/v1/reading-challenges` SHALL return `{items, total}` with every year that has a goal, newest first. Both require authentication and only ever show the caller's own goals.

#### Scenario: Year without a goal
- **WHEN** a reader who never set a goal for 2025 requests 2025
- **THEN** the response has `goal: null` and the number of books read in 2025

### Requirement: Challenge page
The web app SHALL provide `/challenge` «Читательский вызов» with a «Год» selector offering the current year and every year with a goal. Without a goal the card says «Цель на <year> год пока не задана. Вы уже прочитали <n> книг…» and offers a goal field; with a goal it offers «Изменить цель» and «Отмена». The field checks «Минимум 1 книга», «Максимум 10000 книг» and «Цель должна быть целым числом». «История» lists the years with goals. A visitor who is not signed in sees «Необходимо войти в аккаунт, чтобы задать читательский вызов.». The header and footer link to the page for signed-in users.

#### Scenario: Setting the first goal
- **WHEN** a reader without a goal enters 12 and clicks «Сохранить»
- **THEN** the card shows the progress toward 12 and the year appears under «История»
