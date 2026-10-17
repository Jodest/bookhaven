# Proposal

## Why

The home page looks unbalanced: the hero repeats the header (logo, «Каталог», «Мои полки») and shows the first 12 genres alphabetically, which are mostly niche Wikidata genres; «Что почитать?» stretches across the full width with a single select in it; the literary calendar often holds one line next to a tall «Книга дня»; book rows scroll sideways with a visible scrollbar. Issue: #468.

## What Changes

- Remove the hero: heading, tagline, «Найти книгу» / «Мои полки» buttons and genre links. The genre list is still loaded for the «Что почитать?» selector.
- Top of the page: «Продолжить чтение» (signed-in), then a two-column area at `lg`+ — main column «Книга дня» and «Из вашего «Хочу прочитать»», a narrow sidebar with «Читательский вызов», «Что почитать?» and the literary calendar. One column on small screens, same order.
- Home book rows become a one-row grid without horizontal scroll: as many books as fit the width, two rows of two on phones. Payloads are unchanged.

## Capabilities

### Modified Capabilities

- `home/home-page`: «Home page layout».
- `home/personal-blocks`: «Personal blocks on the page».
- `home/catalog-rows`: «New books» (row size is about the payload, the page shows what fits).

## Impact

- backend: none.
- frontend: `apps/web` — `views/home`, `widgets/personal-home`, `features/pick-random-book`, `widgets/literary-calendar`, `entities/book` (`BookRow`).
