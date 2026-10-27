# Proposal

## Why

The «Экранизации» block on the book page takes too much space. Each card spends about nine lines: metadata spread over three lines, a separate «Что лучше?» line, long vote buttons that wrap in the narrow grid columns, and the note «Голосуя, вы отмечаете, что смотрели экранизацию.». The guest and «not read yet» prompts depend only on the book and the visitor, yet they repeat in every card.

## What Changes

- The vote buttons become a compact segmented control on the same line as «Что лучше:», with the short captions «Книга», «Экранизация» and «Одинаково». Behavior is unchanged: clicking the selected one removes the vote.
- The note «Голосуя, вы отмечаете, что смотрели экранизацию.» is removed.
- The guest prompt «Войдите, чтобы сравнить книгу и экранизацию» and the prompt «Отметьте книгу как «Прочитано», чтобы сравнить её с экранизацией.» are shown once under the block heading instead of in every card.
- Card metadata (year, type, director, Kinopoisk and IMDb links) is laid out on one line. This is layout only, so the «Adaptations block on the book page» requirement is unchanged.
- Frontend only. No API or data change.

## Capabilities

### Modified Capabilities

- `screen-works/book-adaptations`: «Voting on the book page» gets short button captions, no note, and prompts once per block.
