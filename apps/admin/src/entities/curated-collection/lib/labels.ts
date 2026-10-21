import type {
  AdminCuratedItemState,
  AutoMetric,
  CollectionMovement,
  CollectionSourceAdaptation,
  CuratedCollectionKind,
} from '@bookhaven/api-client';

/** Marker text of an item that holds no public place; ranked items show their place. */
export const ITEM_STATE_LABELS: Record<Exclude<AdminCuratedItemState, 'ranked'>, string> = {
  reserve: 'Резерв',
  hidden: 'Скрыта: не опубликована',
};

/**
 * Warning shown when fewer books are visible than the collection has places;
 * null when the collection is full.
 */
export function reserveWarning(shown: number, size: number): string | null {
  if (shown >= size) return null;
  return `В подборке видно ${shown} из ${size} книг — пополните резерв`;
}

/** «Ручная» / «Авто» column and label. */
export const KIND_LABELS: Record<CuratedCollectionKind, string> = {
  manual: 'Ручная',
  auto: 'Авто',
};

export const METRIC_LABELS: Record<AutoMetric, string> = {
  weighted_rating: 'Взвешенный рейтинг (оценки читателей)',
  most_read: 'Чаще всего прочитаны',
  most_wanted: 'Чаще всего хотят прочитать',
  trending: 'В тренде (активность за период)',
  book_beats_adaptation: 'Книга лучше экранизации (голоса читателей)',
  adaptation_beats_book: 'Экранизация лучше книги (голоса читателей)',
};

/** Metrics scored by «Книга или экранизация?» votes (score is a 0-1 share). */
export function isVerdictMetric(metric: AutoMetric): boolean {
  return metric === 'book_beats_adaptation' || metric === 'adaptation_beats_book';
}

function votesWord(n: number): string {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return 'голос';
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return 'голоса';
  return 'голосов';
}

/** «Экранизация: «Война и мир» (1967) · 40 голосов»; the year is left out when unknown. */
export function adaptationSourceLabel(adaptation: CollectionSourceAdaptation): string {
  const year = adaptation.year ? ` (${adaptation.year})` : '';
  return `Экранизация: «${adaptation.title}»${year} · ${adaptation.votes_count} ${votesWord(adaptation.votes_count)}`;
}

/**
 * Warning of an automatic collection whose rule matches fewer books than it
 * has places; null when full.
 */
export function shortSnapshotWarning(shown: number, size: number): string | null {
  if (shown >= size) return null;
  return `Правилу соответствуют только ${shown} из ${size} книг`;
}

/** The warning that fits the collection kind (reserve for manual, short snapshot for auto). */
export function shownWarning(kind: CuratedCollectionKind, shown: number, size: number): string | null {
  return kind === 'auto' ? shortSnapshotWarning(shown, size) : reserveWarning(shown, size);
}

/** «↑3» / «↓2» / «Новинка»; null when nothing should be shown. Kept local to admin. */
export function formatAdminMovement(movement: CollectionMovement | null | undefined): string | null {
  if (!movement) return null;
  if (movement.direction === 'up') return `↑${movement.by}`;
  if (movement.direction === 'down') return `↓${movement.by}`;
  if (movement.direction === 'new') return 'Новинка';
  return null;
}

/**
 * Snapshot score: 2 decimals for the weighted rating, a whole percent for the verdict
 * metrics, a whole number otherwise; '' when absent.
 */
export function formatScore(metric: AutoMetric, score: number | null | undefined): string {
  if (score === null || score === undefined) return '';
  if (isVerdictMetric(metric)) return `${Math.round(score * 100)}%`;
  return metric === 'weighted_rating' ? score.toFixed(2) : String(Math.round(score));
}
