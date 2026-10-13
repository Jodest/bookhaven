import type { AdminCurationCandidate, AdminCurationOption } from '@bookhaven/api-client';

/** An option may be ticked unless it is an author rejected earlier. */
export function isOptionSelectable(option: AdminCurationOption): boolean {
  return option.existing?.status !== 'rejected';
}

/**
 * «Выбрать все найденные»: the QID of the single option of every matched
 * candidate whose author is not in the catalog yet. Ambiguous candidates
 * are never auto-ticked.
 */
export function selectAllMatched(candidates: AdminCurationCandidate[]): string[] {
  const out: string[] = [];
  for (const c of candidates) {
    if (c.match_status !== 'matched' || c.options.length !== 1) continue;
    const option = c.options[0];
    if (option.existing) continue;
    if (!out.includes(option.qid)) out.push(option.qid);
  }
  return out;
}

/** Harvest work limit for n authors: 300 each, at most the server clamp 5000. */
export function harvestLimitFor(n: number): number {
  return Math.min(5000, 300 * n);
}
