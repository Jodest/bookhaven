import {
  ApiError,
  type AdminGenreAIBulkApplyReport,
  type AdminGenreAIBulkDismissReport,
  type AdminGenreMergeBatchResult,
} from '@bookhaven/api-client';

// Texts of the AI genre suggestion actions (openspec
// catalog-moderation/ai-genre-suggestions, «Apply and dismiss dialogs» and
// «Bulk actions on the page»).

const STALE_TEXT = 'Предложение устарело: жанры изменились после анализа.';
const NOT_OPEN_TEXT = 'Предложение уже обработано. Обновите список.';

/** Error text of a failed apply (and of a failed merge preview). */
export function applyErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 409) {
      return err.message === 'suggestion is not open' ? NOT_OPEN_TEXT : STALE_TEXT;
    }
    if (err.status === 404) {
      return err.message === 'suggestion not found'
        ? 'Предложение не найдено. Обновите список.'
        : 'Жанр не найден — возможно, его уже слили. Обновите список и попробуйте снова.';
    }
  }
  return 'Не удалось применить предложение. Попробуйте позже.';
}

// " Переключено подборок: N." when a merge switched collections, else "".
const switchedSuffix = (collectionsSwitched: number) =>
  collectionsSwitched > 0 ? ` Переключено подборок: ${collectionsSwitched}.` : '';

export function mergeSuccessMessage(result: AdminGenreMergeBatchResult, targetName: string): string {
  return `Жанры слиты в «${targetName}». Перенесено книг: ${result.books_reassigned}.${switchedSuffix(result.collections_switched)}`;
}

export function junkSuccessMessage(name: string): string {
  return `Жанр «${name}» отклонён.`;
}

export function junkConfirmText(name: string, booksTotal: number): string {
  return `Отклонить жанр «${name}»? Связи с книгами (${booksTotal}) сохранятся, жанр исчезнет со страниц книг.`;
}

export function bulkApplyMessage(report: AdminGenreAIBulkApplyReport): string {
  return `Применено: ${report.applied}. Устарело: ${report.stale}. Ошибок: ${report.errors.length}.`;
}

export const DISMISS_CONFIRM_TEXT = 'Отклонить предложение? ИИ больше не предложит его для тех же жанров.';
export const DISMISSED_MESSAGE = 'Предложение отклонено.';

export function bulkDismissConfirmText(count: number): string {
  return `Отклонить выбранные предложения (${count})? ИИ больше не предложит их для тех же жанров.`;
}

export function bulkDismissMessage(report: AdminGenreAIBulkDismissReport): string {
  return `Отклонено предложений: ${report.dismissed}.`;
}

export function dismissErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 404) return 'Предложение не найдено. Обновите список.';
    if (err.status === 409) return NOT_OPEN_TEXT;
  }
  return 'Не удалось отклонить предложение. Попробуйте позже.';
}

export const ANALYZE_CONFIRM_TEXT = 'Запустить ИИ-анализ жанров? Это займёт несколько минут.';
export const ANALYZE_QUEUED_TEXT = 'Анализ поставлен в очередь. Предложения появятся через несколько минут.';
export const ANALYSIS_DISABLED_TEXT = 'ИИ-анализ жанров выключен.';

export function analyzeErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 409) return 'Анализ уже идёт.';
    if (err.status === 503) return ANALYSIS_DISABLED_TEXT;
  }
  return 'Не удалось запустить анализ. Попробуйте позже.';
}
