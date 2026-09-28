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
