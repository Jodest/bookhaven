import type { AiDraftLine } from '@bookhaven/api-client';

/** One «Название — Автор» line (bulk paste accepts it as is). */
export function proposalText(line: Pick<AiDraftLine, 'title' | 'author'>): string {
  return line.author ? `${line.title} — ${line.author}` : line.title;
}

/** The not_found entries, one per line, ready to be pasted into «Вставить список». */
export function notFoundLines(lines: AiDraftLine[]): string {
  return lines
    .filter((l) => l.status === 'not_found')
    .map(proposalText)
    .join('\n');
}
