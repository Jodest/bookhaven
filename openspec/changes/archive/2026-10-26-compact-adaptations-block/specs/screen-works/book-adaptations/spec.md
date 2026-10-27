## MODIFIED Requirements

### Requirement: Voting on the book page
The «Экранизации» block SHALL offer voting according to the visitor:
- A guest sees the link «Войдите, чтобы сравнить книгу и экранизацию» once, under the block heading, and not in each adaptation. The link leads to the login page and back to the book.
- A signed-in user who may vote sees, under the verdict of each adaptation, «Что лучше:» on the same line as a compact segmented control with the buttons «Книга», «Экранизация» and «Одинаково». No explanatory note is shown. The button of the reader's current vote is shown as selected. Clicking another button changes the vote, and clicking the selected one removes it. The buttons are disabled while a request runs. A failure shows «Не удалось сохранить голос.» and keeps the previous state.
- A signed-in user who may not vote sees «Отметьте книгу как «Прочитано», чтобы сравнить её с экранизацией.» once, under the block heading. When they have a vote on an adaptation, that adaptation shows «Ваш голос: <Книга лучше|Экранизация лучше|Одинаково>» and the button «Убрать голос».

The block-level prompts are not shown while the verdicts load or when they cannot be loaded. A user may vote once they have finished the book at least once, whatever its current status. After a vote changes, the verdict line updates without reloading the page. When the reader changes the book's reading status on the same page, the voting controls and prompts update without a reload.

#### Scenario: Reader votes
- **WHEN** a reader with the book on «Прочитано» clicks «Экранизация» under an adaptation
- **THEN** that button is shown as selected and the verdict line reflects the new vote

#### Scenario: Reader removes the vote
- **WHEN** the reader clicks the selected «Экранизация» again
- **THEN** no button is selected and the vote is removed

#### Scenario: Guest prompt
- **WHEN** a guest opens a book with two adaptations
- **THEN** each adaptation shows its verdict line and no voting buttons, and «Войдите, чтобы сравнить книгу и экранизацию» appears once under the block heading

#### Scenario: Not read yet
- **WHEN** a signed-in user who never finished the book opens a book with two adaptations
- **THEN** «Отметьте книгу как «Прочитано», чтобы сравнить её с экранизацией.» appears once under the block heading, and no adaptation shows voting buttons

#### Scenario: Marking the book as read on the page
- **WHEN** that user marks the book «Прочитано» with the shelf buttons of the same page
- **THEN** the block-level prompt disappears and the voting buttons appear under each adaptation without a reload

#### Scenario: Vote kept after leaving «Прочитано»
- **WHEN** a reader who voted «Книга лучше» and finished the book only once removes it from «Прочитано» and looks at the adaptation
- **THEN** it shows «Ваш голос: Книга лучше» and «Убрать голос», and no voting buttons

#### Scenario: Re-reading keeps the voting buttons
- **WHEN** a reader who voted «Книга лучше» presses «Читаю снова» on the same page
- **THEN** the voting buttons stay, with «Книга» selected
