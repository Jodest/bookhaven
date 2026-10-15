import { ApiError } from '@bookhaven/api-client';
import type { AiDraftErrorCode, AiDraftStatus } from '@bookhaven/api-client';

/** Readable reason for each draft error_code. */
export const DRAFT_ERROR_CODE_LABELS: Record<AiDraftErrorCode, string> = {
  model_refusal: 'ИИ отказался составлять список',
  model_truncated: 'Ответ ИИ оборвался — уменьшите количество книг',
  invalid_output: 'ИИ вернул некорректный ответ',
  model_error: 'Ошибка обращения к ИИ',
  enqueue_failed: 'Не удалось поставить в очередь',
  timeout: 'Черновик не был составлен за час',
};

export function draftErrorCodeLabel(code: AiDraftErrorCode | null): string {
  return code ? DRAFT_ERROR_CODE_LABELS[code] : 'Не удалось составить черновик';
}

export const DRAFT_STATUS_LABELS: Record<AiDraftStatus, string> = {
  queued: 'В очереди',
  running: 'Составляется',
  done: 'Готов',
  failed: 'Не удался',
};

/** Readable Russian message for a failed POST /admin/ai-collections/drafts. */
export function createDraftErrorText(err: unknown): string {
  if (err instanceof ApiError) {
    switch (err.status) {
      case 400:
        return 'Проверьте описание (1–500 символов) и количество книг (1–1000).';
      case 404:
        return 'Подборка не найдена.';
      case 409:
        return 'Черновик для этой подборки уже составляется';
      case 422:
        return 'Черновики ИИ доступны только для ручных подборок.';
      case 429:
        return 'Лимит черновиков на сегодня исчерпан';
      case 502:
        return 'Не удалось поставить черновик в очередь. Попробуйте ещё раз.';
      case 503:
        return 'Черновики ИИ выключены';
    }
    if (err.message) return err.message;
  }
  return 'Не удалось создать черновик.';
}

/** Readable text of a failed bulk add of the chosen draft books. */
export function applyErrorText(err: unknown): string | undefined {
  if (!err) return undefined;
  if (err instanceof ApiError && err.status === 422 && /full/i.test(err.message)) {
    return 'Не хватает мест в подборке (максимум 1000 книг)';
  }
  return err instanceof Error ? err.message : 'Не удалось добавить книги';
}
