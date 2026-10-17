## 1. Frontend

- [x] 1.1 Remove the hero section (heading, tagline, buttons, genre links) from `views/home`; keep loading genres for «Что почитать?»
- [x] 1.2 Split `PersonalHome`: «Продолжить чтение» row, reading-challenge card and want-to-read card as separate client widgets sharing the `/home/me` query
- [x] 1.3 Two-column top area at `lg`+ (main: «Книга дня», «Из вашего «Хочу прочитать»»; sidebar ~340px: «Читательский вызов», «Что почитать?», calendar); single column below `lg`; no empty column when one side has nothing
- [x] 1.4 Compact «Что почитать?» and literary calendar for the sidebar (select and button stacked)
- [x] 1.5 `BookRow`: one-row responsive grid without horizontal scroll (2 rows of 2 on phones), used by every home row
- [x] 1.6 Update/add tests for the changed widgets
- [x] 1.7 Invalidate the home challenge query after saving a goal on `/challenge`
- [x] 1.8 Move every block below «Книга дня» (and «Продолжить чтение») into the main column; keep the mobile order (top blocks → sidebar → rows); narrower BookRow grid for the main column
