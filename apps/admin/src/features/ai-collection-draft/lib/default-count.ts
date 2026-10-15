/** The most books one draft can ask for (server limit). */
export const MAX_DRAFT_COUNT = 1000;

/** Default «Сколько книг предложить»: the collection size plus a fifth, rounded up, at most 1000. */
export function defaultDraftCount(size: number): number {
  const base = Math.max(0, Math.floor(size));
  return Math.min(Math.max(base + Math.ceil(base / 5), 1), MAX_DRAFT_COUNT);
}
