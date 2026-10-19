## MODIFIED Requirements

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
