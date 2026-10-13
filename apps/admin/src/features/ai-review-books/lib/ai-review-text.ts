import { ApiError } from '@bookhaven/api-client';

export const AI_REVIEW_QUEUED_TEXT =
  'Проверка ИИ поставлена в очередь. Вердикты появятся через несколько минут.';

export function aiReviewAllConfirmText(count: number): string {
  return `Отправить на проверку ИИ все непроверенные книги (${count})?`;
}

/** Banner text for a failed trigger call. */
export function aiReviewErrorText(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 409) return 'Проверка ИИ уже в очереди.';
    if (err.status === 503) return 'ИИ-проверка выключена.';
    if (err.message) return err.message;
  }
  return 'Не удалось поставить проверку ИИ в очередь.';
}
