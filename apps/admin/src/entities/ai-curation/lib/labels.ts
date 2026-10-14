import type { AdminCurationStatusValue } from '@bookhaven/api-client';

/**
 * Match badges of a proposed name. «Найден» on purpose, never
 * «Подтверждён»: a match is a Wikidata person with that name, not a
 * confirmation that the model picked the right one.
 */
export const MATCH_STATUS_LABELS: Record<string, string> = {
  matched: 'Найден',
  ambiguous: 'Неоднозначно',
  not_found: 'Не найден',
  error: 'Ошибка Wikidata',
  pending: 'Проверяется…',
};

/** Readable messages for a failed request's error_code. */
export const CURATION_ERROR_MESSAGES: Record<string, string> = {
  model_refusal: 'Модель отказалась отвечать на этот запрос. Переформулируйте его.',
  model_truncated: 'Ответ модели оборвался. Попробуйте запросить меньше авторов.',
  invalid_output: 'Модель вернула ответ в неверном формате. Попробуйте ещё раз.',
  model_error: 'Не удалось получить ответ модели. Попробуйте позже.',
  enqueue_failed: 'Не удалось поставить запрос в очередь. Попробуйте ещё раз.',
  timeout: 'Запрос не завершился за час. Создайте новый.',
};

export const CURATION_STATUS_LABELS: Record<AdminCurationStatusValue, string> = {
  queued: 'В очереди',
  running: 'Выполняется',
  done: 'Готово',
  failed: 'Ошибка',
};

/** Badge text of a match status; an unknown value is shown raw. */
export function matchStatusLabel(status: string): string {
  return MATCH_STATUS_LABELS[status] ?? status;
}

/** Message of an error code; an unknown code is shown raw. */
export function curationErrorMessage(code: string): string {
  return CURATION_ERROR_MESSAGES[code] ?? code;
}

/** Status text of a request; an unknown value is shown raw. */
export function curationStatusLabel(status: string): string {
  return CURATION_STATUS_LABELS[status as AdminCurationStatusValue] ?? status;
}

/** True while the worker still processes the request. */
export function isCurationActive(status: string): boolean {
  return status === 'queued' || status === 'running';
}
