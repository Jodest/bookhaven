import type { AutoMetric, CollectionSourceAdaptation } from '@bookhaven/api-client';
import { pluralRu } from '@/shared/lib/plural-ru';

/**
 * The line under a place of a verdict collection:
 * «Книга лучше экранизации «X» (1967): 90% · 40 голосов». Null for other
 * metrics and for a place whose source adaptation is no longer visible.
 */
export function adaptationLine(
  metric: AutoMetric,
  adaptation: CollectionSourceAdaptation | null | undefined
): string | null {
  if (!adaptation) return null;
  if (metric !== 'book_beats_adaptation' && metric !== 'adaptation_beats_book') return null;
  const year = adaptation.year ? ` (${adaptation.year})` : '';
  const stats = `${Math.round(adaptation.share * 100)}% · ${adaptation.votes_count} ${pluralRu(
    adaptation.votes_count,
    ['голос', 'голоса', 'голосов']
  )}`;
  return metric === 'book_beats_adaptation'
    ? `Книга лучше экранизации «${adaptation.title}»${year}: ${stats}`
    : `Экранизация «${adaptation.title}»${year} лучше книги: ${stats}`;
}
