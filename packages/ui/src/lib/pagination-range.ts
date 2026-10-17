export type PaginationItem =
  | { type: 'page'; page: number }
  | { type: 'ellipsis'; key: 'start' | 'end' };

/**
 * Builds the numbered pager items: first and last page are always present,
 * a window of `siblings` pages surrounds the current one, and an ellipsis
 * only ever replaces two or more pages.
 */
export function getPaginationRange(current: number, total: number, siblings = 1): PaginationItem[] {
  if (!Number.isFinite(total) || total <= 0) return [];
  const last = Math.floor(total);
  const cur = Math.min(Math.max(Math.floor(current) || 1, 1), last);
  const pages = (from: number, to: number): PaginationItem[] =>
    Array.from({ length: Math.max(0, to - from + 1) }, (_, i) => ({
      type: 'page' as const,
      page: from + i,
    }));

  if (last <= 2 * siblings + 5) return pages(1, last);

  const left = Math.max(cur - siblings, 1);
  const right = Math.min(cur + siblings, last);
  const showStartEllipsis = left > 3;
  const showEndEllipsis = right < last - 2;

  if (!showStartEllipsis) {
    return [...pages(1, 2 * siblings + 3), { type: 'ellipsis', key: 'end' }, ...pages(last, last)];
  }
  if (!showEndEllipsis) {
    return [...pages(1, 1), { type: 'ellipsis', key: 'start' }, ...pages(last - (2 * siblings + 2), last)];
  }
  return [
    ...pages(1, 1),
    { type: 'ellipsis', key: 'start' },
    ...pages(left, right),
    { type: 'ellipsis', key: 'end' },
    ...pages(last, last),
  ];
}
