import type { AdaptationVerdict, AdaptationVoteValue } from '@bookhaven/api-client';
import { pluralRu } from '@/shared/lib/plural-ru';

/** Button and «Ваш голос» captions of the three vote values. */
export const ADAPTATION_VOTE_LABELS: Record<AdaptationVoteValue, string> = {
  book: 'Книга лучше',
  adaptation: 'Экранизация лучше',
  same: 'Одинаково',
};

/** Short captions of the segmented vote control. */
export const ADAPTATION_VOTE_SHORT_LABELS: Record<AdaptationVoteValue, string> = {
  book: 'Книга',
  adaptation: 'Экранизация',
  same: 'Одинаково',
};

const LEADER_LABELS: Record<AdaptationVoteValue, string> = {
  book: 'книга лучше',
  adaptation: 'экранизация лучше',
  same: 'одинаково',
};

/**
 * Verdict line of an adaptation: the leader and its share from minVotes votes,
 * «Мнения разделились» on a tie for the lead, null below it (no line is shown).
 */
export function verdictLabel(item: AdaptationVerdict, minVotes: number): string | null {
  const { book_better, adaptation_better, same, votes_count } = item;
  if (votes_count < minVotes || book_better === null || adaptation_better === null || same === null) {
    return null;
  }
  const counts: [AdaptationVoteValue, number][] = [
    ['book', book_better],
    ['adaptation', adaptation_better],
    ['same', same],
  ];
  const top = Math.max(...counts.map(([, n]) => n));
  const leaders = counts.filter(([, n]) => n === top);
  const votes = `${votes_count} ${pluralRu(votes_count, ['голос', 'голоса', 'голосов'])}`;
  if (leaders.length > 1) return `Мнения разделились · ${votes}`;
  const percent = Math.round((top / votes_count) * 100);
  return `${percent}% читателей: ${LEADER_LABELS[leaders[0][0]]} · ${votes}`;
}
