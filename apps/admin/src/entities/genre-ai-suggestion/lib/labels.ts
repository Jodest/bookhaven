import type {
  AdminGenreAIReasonCode,
  AdminGenreAIRun,
  AdminGenreAIRunStatus,
  AdminGenreAISuggestionKind,
  AdminGenreAISuggestionStatus,
} from '@bookhaven/api-client';

/** Kind badge of a suggestion. */
export const KIND_LABELS: Record<AdminGenreAISuggestionKind, string> = {
  duplicate: 'Дубликат',
  narrow: 'Узкий → общий',
  junk: 'Не жанр',
};

/** Options of the kind filter, in display order ('' = any kind). */
export const KIND_FILTER_LABELS: Record<'' | AdminGenreAISuggestionKind, string> = {
  '': 'Все типы',
  duplicate: 'Дубликаты',
  narrow: 'Узкий → общий',
  junk: 'Не жанр',
};

export const SUGGESTION_KINDS: AdminGenreAISuggestionKind[] = ['duplicate', 'narrow', 'junk'];

/** Status tabs, in display order. */
export const STATUS_TAB_LABELS: Record<AdminGenreAISuggestionStatus, string> = {
  open: 'Открытые',
  applied: 'Применённые',
  dismissed: 'Отклонённые',
  stale: 'Устаревшие',
};

export const SUGGESTION_STATUSES: AdminGenreAISuggestionStatus[] = ['open', 'applied', 'dismissed', 'stale'];

export const REASON_CODE_LABELS: Record<AdminGenreAIReasonCode, string> = {
  spelling: 'написание',
  language: 'другой язык',
  synonym: 'синоним',
  too_narrow: 'слишком узкий',
  not_a_genre: 'не жанр',
  date: 'дата',
  marketing: 'маркетинг',
  format: 'формат издания',
};

export const RUN_STATUS_LABELS: Record<AdminGenreAIRunStatus, string> = {
  queued: 'в очереди',
  running: 'идёт',
  done: 'завершён',
  failed: 'завершился с ошибкой',
};

/** Label of a kind; an unknown value is shown raw. */
export function suggestionKindLabel(kind: string): string {
  return KIND_LABELS[kind as AdminGenreAISuggestionKind] ?? kind;
}

/** Label of a reason code; an unknown code is shown raw. */
export function reasonCodeLabel(code: string): string {
  return REASON_CODE_LABELS[code as AdminGenreAIReasonCode] ?? code;
}

/** Type guards for filter values from the URL. */
export function isSuggestionStatus(value: string | null): value is AdminGenreAISuggestionStatus {
  return value !== null && (SUGGESTION_STATUSES as string[]).includes(value);
}

export function isSuggestionKind(value: string | null): value is AdminGenreAISuggestionKind {
  return value !== null && (SUGGESTION_KINDS as string[]).includes(value);
}

/** The action line of a row: «Слить в «{target}»» or «Отклонить жанр». */
export function suggestionActionLabel(kind: AdminGenreAISuggestionKind, targetName: string | null): string {
  return kind === 'junk' ? 'Отклонить жанр' : `Слить в «${targetName ?? '…'}»`;
}

/** Confidence 0..1 as whole percent, e.g. 0.934 -> «93%». */
export function confidencePercent(confidence: number): string {
  return `${Math.round(confidence * 100)}%`;
}

/**
 * Status line of the page: «Последний анализ: {date}» with «идёт…» while a run
 * is queued or running and «завершился с ошибкой» for a failed run; "" when
 * nothing ever ran (the page then shows «Анализ ещё не запускался.»).
 */
export function lastRunText(run: AdminGenreAIRun | null, formatDate: (iso: string) => string): string {
  if (!run) return 'Анализ ещё не запускался.';
  if (run.status === 'queued' || run.status === 'running') {
    return `Последний анализ: ${formatDate(run.created_at)}, идёт…`;
  }
  const date = formatDate(run.finished_at ?? run.created_at);
  if (run.status === 'failed') return `Последний анализ: ${date}, завершился с ошибкой`;
  return `Последний анализ: ${date}`;
}
