import { ApiError } from '@bookhaven/api-client';

/** Shown after a generation was queued. */
export const GENERATE_QUEUED_TEXT = 'Генерация поставлена в очередь.';

/** Confirm before regenerating an approved description. */
export const REGENERATE_APPROVED_CONFIRM =
  'Одобренное описание будет снято с сайта до проверки нового. Продолжить?';

/** Confirm of «Сгенерировать для книг без описания». */
export function generateMissingConfirmText(missing: number, maxBulk: number): string {
  return `Сгенерировать описания ИИ для книг без описания (до ${maxBulk} из ${missing})?`;
}

/** Readable Russian message for a failed POST /admin/ai-descriptions/generate. */
export function generateErrorText(err: unknown): string {
  if (err instanceof ApiError) {
    switch (err.status) {
      case 400:
        return 'Неверный запрос на генерацию.';
      case 409:
        return 'Генерация уже в очереди.';
      case 502:
        return 'Не удалось поставить генерацию в очередь. Попробуйте ещё раз.';
      case 503:
        return 'Генерация описаний ИИ выключена.';
    }
  }
  return 'Не удалось запустить генерацию.';
}
