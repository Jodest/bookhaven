# Proposal

## Why
User feedback on `add-adaptation-posters-and-tabs`: readers do not click tabs, so reviews and similar books hidden behind tabs go unseen, and the 2:3 poster cards (about 200×300 px) are too big. The book page goes back to stacked sections, and adaptations become compact horizontal cards.

## What Changes
- **BREAKING (UI only, no prod links exist):** the tabs «Экранизации (N) · Рецензии · Похожие книги» are removed and the `?tab=` address parameter is no longer handled. It is ignored when present.
- The right column shows «Экранизации», then «Похожие книги», then «Рецензии» one under another, all visible at once. Each has a visible heading.
- Adaptation cards become compact horizontal cards. Each has a 2:3 thumbnail about 72×108 px on the left. On the right are the title, the metadata line, the verdict, «Что лучше:» and a compact «Постер: …» caption. The grid has one column on narrow screens and two on wide ones.
- The first 4 adaptations are shown, then «Показать все (N)» reveals the rest. All cards stay in the server-rendered HTML.
- The guest and not-read voting prompts appear once under the «Экранизации» heading.
- «Похожие книги» becomes a single horizontally scrolling row of small covers with titles. It is a keyboard-focusable list and is hidden when empty or when it fails to load.
- The header link «N экранизаций» becomes an in-page anchor `#adaptations`. The review links in the rating summary go to `#reviews` again.
- The author page «Фильмы об авторе» uses the same compact card.

## Capabilities
### New Capabilities
- none

### Modified Capabilities
- `screen-works/book-adaptations`: tabs removed. The adaptations block, verdict, voting, header link and poster attribution are reworded for a stacked section with compact cards and «Показать все (N)».
- `screen-works/author-films`: the films block uses the compact card.
- `catalog/book-page`: «Page layout» describes the stacked sections, the similar-books row, the anchors and the ignored `tab` parameter.

## Impact
- backend: none (no API or DB change).
- frontend: apps/web only. This covers the book-detail view and page route, the screen-work entity cards, the similar-books widget, the reviews-section widget (prop cleanup) and the author-detail view (through the shared card).
- devops: none.
- docs: ROADMAP entry of the adaptations epic.
