# Self-hosted fonts

Fonts are committed here and loaded via `next/font/local` instead of
`next/font/google`, because the latter performs a live network fetch to
`fonts.gstatic.com` during `next build`, which is unreliable in CI (see
`docs/features/self-hosted-fonts.md`).

## Inter

- Source: [`google/fonts`](https://github.com/google/fonts), path
  `ofl/inter/Inter[opsz,wght].ttf`, commit
  `e1d6480102fed30739fead0faee463101f892c8f` (2024-05-28).
- Variable font, weight axis 100–900, includes Latin and Cyrillic glyphs.
- Converted from `.ttf` to `.woff2` with `fonttools` (`fonttools ttLib.woff2 compress`)
  for smaller size; no glyphs were subset or otherwise modified.
- License: `inter/OFL.txt` (SIL Open Font License 1.1).

## Literata

- Source: [`google/fonts`](https://github.com/google/fonts), path
  `ofl/literata/Literata[opsz,wght].ttf`, commit
  `c8018e986708f2aed63c928b1e9026826da1553d` (2023-05-19).
- Variable font, weight axis 200–900 (used at 400/500/600/700 via
  `next/font/local`'s `weight: '400 700'` range), includes Latin and
  Cyrillic glyphs.
- Converted from `.ttf` to `.woff2` with `fonttools` (`fonttools ttLib.woff2 compress`)
  for smaller size; no glyphs were subset or otherwise modified.
- License: `literata/OFL.txt` (SIL Open Font License 1.1).
