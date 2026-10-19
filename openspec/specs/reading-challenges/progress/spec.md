# reading-challenges/progress Specification

## Purpose

Shows a reader how far they are toward their yearly goal, counted from what they have marked as read.

## Requirements

### Requirement: Counting read books
The books read in a year SHALL be the distinct books on the caller's «Прочитано» shelf whose time of adding to that shelf falls within the calendar year (UTC). The count is computed live on every request: a book removed from «Прочитано», or moved to another default shelf, stops counting, and a book that comes back to it counts in the year it came back. Books are counted whatever their publication status.

#### Scenario: Book moved back to «Читаю»
- **WHEN** a reader moves a book from «Прочитано» back to «Читаю»
- **THEN** the year's count goes down by one

#### Scenario: Shelved last year
- **WHEN** a book was added to «Прочитано» on 31 December of last year (UTC)
- **THEN** it counts for last year, not this year

### Requirement: Progress figures
The progress SHALL be `{year, goal, books_read, progress_percent, completed}`. With a goal, `progress_percent` is books read divided by the goal, rounded to a whole percent and capped at 100, and `completed` is true once books read reach the goal. Without a goal, `progress_percent` is `null` and `completed` is false.

#### Scenario: Goal exceeded
- **WHEN** a reader with a goal of 10 has read 13 books this year
- **THEN** `progress_percent` is 100 and `completed` is true

### Requirement: Progress on the site
The challenge page SHALL show «Прочитано <n> из <goal> книг (<percent>%)», a progress bar, «Цель на <year> год достигнута!» when completed, and the note that progress counts books on «Прочитано» for that year. The home page card «Читательский вызов» for signed-in users shows «Прочитано <n> из <goal> в <year>» with a bar and «Подробнее», or, without a goal, «Поставьте цель на <year> год и следите, сколько книг уже прочитано.» with «Поставить цель». The figures on the home card MUST reflect every shelf change the reader made earlier in the same visit (on the book page, on book cards or on their shelf pages), without a page reload and without waiting for a cache to expire.

#### Scenario: Home card without a goal
- **WHEN** a signed-in reader without a goal for this year opens `/`
- **THEN** the challenge card offers «Поставить цель» linking to `/challenge`

#### Scenario: Home card after marking a book read
- **WHEN** a reader with a goal sees «Прочитано 5 из 20 в <year>» on `/`, opens a book page, picks «Прочитано» and goes back to `/` without reloading the page
- **THEN** the challenge card shows «Прочитано 6 из 20 в <year>»

#### Scenario: Home card after removing a book from «Прочитано»
- **WHEN** a reader removes a book added this year from their «Прочитано» shelf with «Убрать с полки» on the shelf page and then opens `/` without reloading the page
- **THEN** the challenge card shows one book fewer than before
