import { ApiError } from '@bookhaven/api-client';

/** Readable Russian message for a failed POST /admin/ai-curator/requests. */
export function createCurationErrorText(err: unknown): string {
  if (err instanceof ApiError) {
    switch (err.status) {
      case 400:
        return 'Проверьте запрос: от 1 до 500 символов и от 1 до 30 авторов.';
      case 409:
        return 'У вас уже есть подборка в работе. Дождитесь её завершения.';
      case 429:
        return 'Достигнут дневной лимит подборок (20 за 24 часа). Попробуйте позже.';
      case 502:
        return 'Не удалось поставить подборку в очередь. Попробуйте ещё раз.';
      case 503:
        return 'Подбор ИИ выключен.';
    }
    if (err.message) return err.message;
  }
  return 'Не удалось создать подборку.';
}
