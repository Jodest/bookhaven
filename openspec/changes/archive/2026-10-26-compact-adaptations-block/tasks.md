# Tasks

- [x] 1. `BookAdaptations`: put the year, type, «Режиссёр: …» and the Kinopoisk/IMDb links on one wrapping line separated by « · ». Add a slot that renders once under the heading (for the voting prompt).
- [x] 2. `AdaptationVerdict`: «Что лучше:» on the same line as a segmented control (joined buttons) with the captions «Книга», «Экранизация», «Одинаково». Keep aria-pressed, toggle-off and disabled-while-pending. Remove the note.
- [x] 3. Move the guest and «not read yet» prompts out of each card into a block-level component of `features/vote-adaptation`, rendered once through the new slot. It renders nothing while the verdicts load or after they fail. Cards then show only the verdict line in those states. «Ваш голос: … / Убрать голос» stays per card.
- [x] 4. Update the tests (`adaptation-verdict.test.tsx`, `book-adaptations.test.tsx`, `book-detail-view.test.tsx`) for the new captions, the removed note and the prompts appearing once.
